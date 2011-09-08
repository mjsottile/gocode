//
// s-expression parser, based on Rob Pike's 2011 lexical scanning in go
// talk.
//
// matt@galois.com // sept. 2011
//
package main

import "utf8"
import "fmt"
import "strings"

const eof = -1

// s-expression lexer item
type item struct {
  typ itemType
  val string
}

// s-expression structure item
type sexpr struct {
  aty atomType
  sty sexprType
  next *sexpr
  list *sexpr
  val  string
}

type itemType  int
type atomType  int
type sexprType int

const (
  itemError itemType = iota
  itemRParen
  itemLParen
  itemEOF
  itemAtom
  itemDQAtom
)

const (
  sexprAtom sexprType = iota
  sexprList
)

const (
  atomBasic atomType = iota
  atomDQuote
  atomInvalid
)

func (i item) String() string {
  switch i.typ {
    case itemEOF:
      return "EOF"
    case itemError:
      return i.val
  }
  if len(i.val) > 20 {
    return fmt.Sprintf("%d:%.20q...", i.typ, i.val)
  }
  return fmt.Sprintf("%d:%q", i.typ, i.val)
}

func (s sexpr) String() string {
  switch s.sty {
    case sexprList:
      return fmt.Sprintf("LIST:\n  next=%s\n  list=%s\n",s.next,s.list)
    case sexprAtom:
      return fmt.Sprintf("%s -> %s",s.val,s.next)
  }
  return ""
}

type stateFn func(*lexer) stateFn

func parse (ch chan item) (* sexpr) {
  i := <-ch
  switch i.typ {
    case itemLParen:
      slist := parse (ch)
      snext := parse (ch)
      s := &sexpr { aty  : atomInvalid,
                    sty  : sexprList,
                    val  : "",
                    list : slist,
                    next : snext }
      return s
    case itemRParen:
      return nil
    case itemAtom:
      snext := parse (ch)
      s := &sexpr { aty  : atomBasic,
                    sty  : sexprAtom, 
                    val  : i.val, 
                    list : nil,
                    next : snext }
      return s
    case itemDQAtom:
  }
  return nil
}

type lexer struct {
  name  string
  input string
  start int
  pos   int
  width int
  items chan item
}

func lex(name, input string) (*lexer, chan item) {
  l := &lexer{
       name : name,
       input: input,
       items: make(chan item),
  }

  go l.run()

  return l, l.items
}

// 
func (l *lexer) run() {
  for state := lexAtom; state != nil; {
    state = state(l)
  }
  close(l.items)
}

func (l *lexer) emit(t itemType) {
  l.items <- item{t, l.input[l.start:l.pos]}
  l.start = l.pos
}

func lexAtom(l *lexer) stateFn {
 for {
   if l.peek() == '(' {
     if (l.pos > l.start) {
       l.emit(itemAtom)
     }
     return lexLeftParen
   }
   if l.peek() == ')' {
     if (l.pos > l.start) {
       l.emit(itemAtom)
     }
     return lexRightParen
   }
   if l.peek() == '"' {
     if (l.pos > l.start) {
       l.emit(itemAtom)
     }
     l.next()
     return lexDQuote
   }
   if l.peek() == ' '  || l.peek() == '\t' || 
      l.peek() == '\r' || l.peek() == '\n' {
     if (l.pos > l.start) {
       l.emit(itemAtom)
     }
     return lexWhitespace
   }
   if l.next() == eof { break }
 }
 if l.pos > l.start {
   l.emit(itemAtom)
 }
 l.emit(itemEOF)
 return nil
}

func lexDQuote(l *lexer) stateFn {
  if l.accept("\"") {
    l.emit(itemDQAtom)
    return lexAtom
  }
  l.next()
  return lexDQuote
}

func (l *lexer) accept(valid string) bool {
    if strings.IndexRune(valid, l.next()) >= 0 {
        return true
    }
    l.backup()
    return false
}

func lexWhitespace(l *lexer) stateFn {
  whitespace := " \r\n\t"
  if l.accept(whitespace) {
    l.ignore()
    return lexWhitespace
  }
  return lexAtom
}

func lexLeftParen(l *lexer) stateFn {
  l.pos += len(leftParen)
  l.emit(itemLParen)
  return lexAtom
}

func lexRightParen(l *lexer) stateFn {
  l.pos += len(rightParen)
  l.emit(itemRParen)
  return lexAtom
}

func (l *lexer) ignore() {
  l.start = l.pos
}

func (l *lexer) backup() {
  l.pos -= l.width
}

func (l *lexer ) peek() int {
  rune := l.next()
  l.backup()
  return rune
}

func (l *lexer) next() (rune int) {
  if l.pos >= len(l.input) {
    l.width = 0
    return eof
  }
  rune, l.width =
    utf8.DecodeRuneInString(l.input[l.pos:])
  l.pos += l.width
  return rune
}

const leftParen  = "("
const rightParen = ")"

func printall(ch chan item) {
  for { 
    i := <- ch
    fmt.Println(i) 
    if i.typ == itemEOF {
      break
    }
  }
}

// main will be for testing for now
func main() {
  l, items := lex("S-Expression Lexer","(test (test2 \"i am long\" test3) blah)")
  s := parse(items)
  fmt.Println(l.name)
//  printall(items)
  fmt.Println(s)
}