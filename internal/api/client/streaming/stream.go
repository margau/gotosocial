// GoToSocial
// Copyright (C) GoToSocial Authors admin@gotosocial.org
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package streaming

import (
	"context"
	"time"

	"codeberg.org/gruf/go-kv"
	apiutil "github.com/superseriousbusiness/gotosocial/internal/api/util"
	"github.com/superseriousbusiness/gotosocial/internal/gtserror"
	"github.com/superseriousbusiness/gotosocial/internal/gtsmodel"
	"github.com/superseriousbusiness/gotosocial/internal/log"
	"github.com/superseriousbusiness/gotosocial/internal/oauth"
	streampkg "github.com/superseriousbusiness/gotosocial/internal/stream"
	"golang.org/x/exp/slices"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// StreamGETHandler swagger:operation GET /api/v1/streaming streamGet
//
// Initiate a websocket connection for live streaming of statuses and notifications.
//
// The scheme used should *always* be `wss`. The streaming basepath can be viewed at `/api/v1/instance`.
//
// On a successful connection, a code `101` will be returned, which indicates that the connection is being upgraded to a secure websocket connection.
//
// As long as the connection is open, various message types will be streamed into it.
//
// GoToSocial will ping the connection every 30 seconds to check whether the client is still receiving.
//
// If the ping fails, or something else goes wrong during transmission, then the connection will be dropped, and the client will be expected to start it again.
//
//	---
//	tags:
//	- streaming
//
//	produces:
//	- application/json
//
//	schemes:
//	- wss
//
//	parameters:
//	-
//		name: access_token
//		type: string
//		description: Access token for the requesting account.
//		in: query
//		required: true
//	-
//		name: stream
//		type: string
//		description: |-
//			Type of stream to request.
//
//			Options are:
//
//			`user`: receive updates for the account's home timeline.
//			`public`: receive updates for the public timeline.
//			`public:local`: receive updates for the local timeline.
//			`hashtag`: receive updates for a given hashtag.
//			`hashtag:local`: receive local updates for a given hashtag.
//			`list`: receive updates for a certain list of accounts.
//			`direct`: receive updates for direct messages.
//		in: query
//		required: true
//	-
//		name: list
//		type: string
//		description: |-
//			ID of the list to subscribe to.
//			Only used if stream type is 'list'.
//		in: query
//	-
//		name: tag
//		type: string
//		description: |-
//			Name of the tag to subscribe to.
//			Only used if stream type is 'hashtag' or 'hashtag:local'.
//		in: query
//
//	security:
//	- OAuth2 Bearer:
//		- read:streaming
//
//	responses:
//		'101':
//			schema:
//				type: object
//				properties:
//					stream:
//						type: array
//						items:
//							type: string
//							enum:
//							- user
//							- public
//							- public:local
//							- hashtag
//							- hashtag:local
//							- list
//							- direct
//					event:
//						description: |-
//							The type of event being received.
//
//							`update`: a new status has been received.
//							`notification`: a new notification has been received.
//							`delete`: a status has been deleted.
//							`filters_changed`: not implemented.
//						type: string
//						enum:
//						- update
//						- notification
//						- delete
//						- filters_changed
//					payload:
//						description: |-
//							The payload of the streamed message.
//							Different depending on the `event` type.
//
//							If present, it should be parsed as a string.
//
//							If `event` = `update`, then the payload will be a JSON string of a status.
//							If `event` = `notification`, then the payload will be a JSON string of a notification.
//							If `event` = `delete`, then the payload will be a status ID.
//						type: string
//						example: "{\"id\":\"01FC3TZ5CFG6H65GCKCJRKA669\",\"created_at\":\"2021-08-02T16:25:52Z\",\"sensitive\":false,\"spoiler_text\":\"\",\"visibility\":\"public\",\"language\":\"en\",\"uri\":\"https://gts.superseriousbusiness.org/users/dumpsterqueer/statuses/01FC3TZ5CFG6H65GCKCJRKA669\",\"url\":\"https://gts.superseriousbusiness.org/@dumpsterqueer/statuses/01FC3TZ5CFG6H65GCKCJRKA669\",\"replies_count\":0,\"reblogs_count\":0,\"favourites_count\":0,\"favourited\":false,\"reblogged\":false,\"muted\":false,\"bookmarked\":fals…//gts.superseriousbusiness.org/fileserver/01JNN207W98SGG3CBJ76R5MVDN/header/original/019036W043D8FXPJKSKCX7G965.png\",\"header_static\":\"https://gts.superseriousbusiness.org/fileserver/01JNN207W98SGG3CBJ76R5MVDN/header/small/019036W043D8FXPJKSKCX7G965.png\",\"followers_count\":33,\"following_count\":28,\"statuses_count\":126,\"last_status_at\":\"2021-08-02T16:25:52Z\",\"emojis\":[],\"fields\":[]},\"media_attachments\":[],\"mentions\":[],\"tags\":[],\"emojis\":[],\"card\":null,\"poll\":null,\"text\":\"a\"}"
//		'401':
//			description: unauthorized
//		'400':
//			description: bad request
func (m *Module) StreamGETHandler(c *gin.Context) {

	// First we check for a query param provided access token
	token := c.Query(AccessTokenQueryKey)
	if token == "" {
		// Else we check the HTTP header provided token
		token = c.GetHeader(AccessTokenHeader)
	}

	var account *gtsmodel.Account
	if token != "" {
		// Check the explicit token
		var errWithCode gtserror.WithCode
		account, errWithCode = m.processor.Stream().Authorize(c.Request.Context(), token)
		if errWithCode != nil {
			apiutil.ErrorHandler(c, errWithCode, m.processor.InstanceGetV1)
			return
		}
	} else {
		// If no explicit token was provided, try regular oauth
		auth, errStr := oauth.Authed(c, true, true, true, true)
		if errStr != nil {
			err := gtserror.NewErrorUnauthorized(errStr, errStr.Error())
			apiutil.ErrorHandler(c, err, m.processor.InstanceGetV1)
			return
		}
		account = auth.Account
	}

	// Get the initial stream type, if there is one.
	// By appending other query params to the streamType,
	// we can allow for streaming for specific list IDs
	// or hashtags.
	streamType := c.Query(StreamQueryKey)
	if list := c.Query(StreamListKey); list != "" {
		streamType += ":" + list
	} else if tag := c.Query(StreamTagKey); tag != "" {
		streamType += ":" + tag
	}

	stream, errWithCode := m.processor.Stream().Open(c.Request.Context(), account, streamType)
	if errWithCode != nil {
		apiutil.ErrorHandler(c, errWithCode, m.processor.InstanceGetV1)
		return
	}

	l := log.WithContext(c.Request.Context()).
		WithFields(kv.Fields{
			{"account", account.Username},
			{"streamID", stream.ID},
			{"streamType", streamType},
		}...)

	// Upgrade the incoming HTTP request, which hijacks the underlying
	// connection and reuses it for the websocket (non-http) protocol.
	wsConn, err := m.wsUpgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		l.Errorf("error upgrading websocket connection: %v", err)
		close(stream.Hangup)
		return
	}

	go func() {
		// We perform the main websocket send loop in a separate
		// goroutine in order to let the upgrade handler return.
		// This prevents the upgrade handler from holding open any
		// throttle / rate-limit request tokens which could become
		// problematic on instances with multiple users.
		l.Info("opened websocket connection")
		defer l.Info("closed websocket connection")

		// Create new context for lifetime of the connection
		ctx, cncl := context.WithCancel(context.Background())

		// Create ticker to send alive pings
		pinger := time.NewTicker(m.dTicker)

		defer func() {
			// Signal done
			cncl()

			// Close websocket conn
			_ = wsConn.Close()

			// Close processor stream
			close(stream.Hangup)

			// Stop ping ticker
			pinger.Stop()
		}()

		go func() {
			// Signal done
			defer cncl()

			for {
				// We have to listen for received websocket messages in
				// order to trigger the underlying wsConn.PingHandler().
				//
				// Read JSON objects from the client and act on them
				var msg map[string]string
				err := wsConn.ReadJSON(&msg)
				if err != nil {
					if ctx.Err() == nil {
						// Only log error if the connection was not closed
						// by us. Uncanceled context indicates this is the case.
						l.Errorf("error reading from websocket: %v", err)
					}
					return
				}
				l.Tracef("received message from websocket: %v", msg)

				// If the message contains 'stream' and 'type' fields, we can
				// update the set of timelines that are subscribed for events.
				updateType, ok := msg["type"]
				if !ok {
					l.Warn("'type' field not provided")
					continue
				}

				updateStream, ok := msg["stream"]
				if !ok {
					l.Warn("'stream' field not provided")
					continue
				}

				// Ignore if the updateStreamType is unknown (or missing),
				// so a bad client can't cause extra memory allocations
				if !slices.Contains(streampkg.AllStatusTimelines, updateStream) {
					l.Warnf("unknown 'stream' field: %v", msg)
					continue
				}

				updateList, ok := msg["list"]
				if ok {
					updateStream += ":" + updateList
				}

				switch updateType {
				case "subscribe":
					stream.Lock()
					stream.StreamTypes[updateStream] = true
					stream.Unlock()
				case "unsubscribe":
					stream.Lock()
					delete(stream.StreamTypes, updateStream)
					stream.Unlock()
				default:
					l.Warnf("invalid 'type' field: %v", msg)
				}
			}
		}()

		for {
			select {
			// Connection closed
			case <-ctx.Done():
				return

			// Received next stream message
			case msg := <-stream.Messages:
				l.Tracef("sending message to websocket: %+v", msg)
				if err := wsConn.WriteJSON(msg); err != nil {
					l.Debugf("error writing json to websocket: %v", err)
					return
				}

				// Reset on each successful send.
				pinger.Reset(m.dTicker)

			// Send keep-alive "ping"
			case <-pinger.C:
				l.Trace("pinging websocket ...")
				if err := wsConn.WriteMessage(
					websocket.PingMessage,
					[]byte{},
				); err != nil {
					l.Debugf("error writing ping to websocket: %v", err)
					return
				}
			}
		}
	}()
}
