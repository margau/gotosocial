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

package text

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// plaintextParser implements goldmark.parser.BlockParser
type plaintextParser struct{}

var defaultPlaintextParser = &plaintextParser{}

func newPlaintextParser() parser.BlockParser {
	return defaultPlaintextParser
}

func (b *plaintextParser) Trigger() []byte {
	return nil
}

func (b *plaintextParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	_, segment := reader.PeekLine()
	node := ast.NewParagraph()
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return node, parser.NoChildren
}

func (b *plaintextParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, segment := reader.PeekLine()
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return parser.Continue | parser.NoChildren
}

func (b *plaintextParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (b *plaintextParser) CanInterruptParagraph() bool {
	return false
}

func (b *plaintextParser) CanAcceptIndentedLine() bool {
	return true
}

// plaintextParserNoParagraph implements goldmark.parser.BlockParser
type plaintextParserNoParagraph struct{}

var defaultPlaintextParserNoParagraph = &plaintextParserNoParagraph{}

func newPlaintextParserNoParagraph() parser.BlockParser {
	return defaultPlaintextParserNoParagraph
}

func (b *plaintextParserNoParagraph) Trigger() []byte {
	return nil
}

func (b *plaintextParserNoParagraph) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	_, segment := reader.PeekLine()
	node := ast.NewDocument()
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return node, parser.NoChildren
}

func (b *plaintextParserNoParagraph) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, segment := reader.PeekLine()
	node.Lines().Append(segment)
	reader.Advance(segment.Len() - 1)
	return parser.Continue | parser.NoChildren
}

func (b *plaintextParserNoParagraph) Close(node ast.Node, reader text.Reader, pc parser.Context) {}

func (b *plaintextParserNoParagraph) CanInterruptParagraph() bool {
	return false
}

func (b *plaintextParserNoParagraph) CanAcceptIndentedLine() bool {
	return true
}
