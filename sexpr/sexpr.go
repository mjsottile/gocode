/*
    Package sexpr implements a library for simplified
    LISP-style symbolic expressions.

    based on Rob Pike's 2011 lexical scanning in go talk.

    matt@galois.com // sept. 2011
*/
package main

import (
  "utf8"
  "fmt"
  "strings"
  "os"
)

/*
   types
*/

// lexer item type
type itemType  int

// s-expression atom type
type atomType  int

// s-expression element type
type sexprType int

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

// lexer context
type lexer struct {
    name  string
    input string
    start int
    pos   int
    width int
    items chan item
}

// state function, concept borrowed from pike talk
type stateFn func(*lexer) stateFn

/*
   constants
*/

// lexer item types
const (
    itemError itemType = iota
    itemRParen
    itemLParen
    itemEOF
    itemAtom
)

// s-expression element types : atoms or lists
const (
    sexprAtom sexprType = iota
    sexprList
)

// s-expression atom types.  currently only one useful type, but later we
// can expand to explicltly distinguish double and single quoted atoms
const (
    atomBasic atomType = iota
    atomInvalid
)

// eof
const eof = -1

/*
   functions
*/

// given an s-expression and a channel, emit a sequence of characters
// representing the unparsed s-expression
func sexprUnparse (s *sexpr, ch chan byte) {
    if (s == nil) {
        return
    }

    for s != nil {
        switch s.sty {
        case sexprList:
            ch <- '('
            sexprUnparse(s.list, ch)
            ch <- ')'
        case sexprAtom:
            for i := range s.val {
                ch <- s.val[i]
            }
        }
        if (s.next != nil) {
            ch <- ' '
            sexprUnparse(s.next, ch)
        }
    }
}

// dump an s-expression to a graphviz dot represenation to look at
func sexprToDotFile (s *sexpr, filename string) {
    file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC,0644)
    if err != nil {
        panic("Error opening file")
    } else {
        fmt.Fprintf(file,"digraph sexp {\n")
        _sexprToDotFile(s,file,1);
        fmt.Fprintf(file,"}\n")
    }
}

// helper used by sexprToDotFile that does the actual IO, and threads a
// counter through so that we can uniquely name the s-expression elements
// in the graphviz output
func _sexprToDotFile(s *sexpr, file *os.File, id int) int {
    fmt.Fprintf(file,"  sx%d [shape=record,label=\"", id)
    switch s.sty {
    case sexprAtom:
        fmt.Fprintf(file,"<type> ATOM value=%s ",s.val)
        break
    case sexprList:
        fmt.Fprintf(file,"<type> LIST")
        break
    default:
        panic("Noooooo!")
    }
    
    fmt.Fprintf(file,"| <list> list | <next> next\"];\n")
    if (s.sty == sexprAtom) {
        if (s.next != nil) {
            next_id := _sexprToDotFile(s.next, file, id+1)
            fmt.Fprintf(file,"  sx%d:next -> sx%d:type;\n", id, id+1)
            return next_id+1;
        }
        return id+1;
    } else {
        if (s.list != nil) {
            list_id := _sexprToDotFile(s.list, file, id+1)
            fmt.Fprintf(file,"  sx%d:list -> sx%d:type;\n", id, id+1)
            if (s.next != nil) {
                next_id := _sexprToDotFile(s.next, file, list_id)
                fmt.Fprintf(file,"  sx%d:next -> sx%d:type;\n", id, list_id)
                return next_id+1;
            }
        } else {
            if (s.next != nil) {
                next_id := _sexprToDotFile(s.next, file, id+1)
                fmt.Fprintf(file,"  sx%d:next -> sx%d:type;\n", id, id+1)
                return next_id+1;
            }
        }
        return id+1;
    }
    return id;
}

// pretty printer for lexer items
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

// pretty printer for s-expression structures.  "pretty" is debatable...
func (s sexpr) String() string {
    switch s.sty {
    case sexprList:
        return fmt.Sprintf("LIST:\n  next=%s\n  list=%s\n",s.next,s.list)
    case sexprAtom:
        return fmt.Sprintf("%s -> %s",s.val,s.next)
    }
    return ""
}

// given a channel of lexer items, parse them into a s-expression structure
func parse (ch chan item) (* sexpr) {
    i := <-ch

    switch i.typ {
    case itemLParen:
        slist := parse (ch)
        snext := parse (ch)
        s := &sexpr { 
          aty  : atomInvalid,
          sty  : sexprList,
          val  : "",
          list : slist,
          next : snext }
        return s
    case itemRParen:
        return nil
    case itemAtom:
        snext := parse (ch)
        s := &sexpr { 
          aty  : atomBasic,
          sty  : sexprAtom, 
          val  : i.val, 
          list : nil,
          next : snext }
        return s
    case itemEOF:
         return nil
    default:
        panic("Bad lex item type")
    }
    return nil
}

// lexer that fires off a go-routine that lexes the input string and
// emits items into a channel
func lex(name, input string) (*lexer, chan item) {
    l := &lexer{
      name : name,
      input: input,
      items: make(chan item),
    }
    
    go l.run()
    
    return l, l.items
}

// body of lexer go-routine that just spins until the current state function
// becomes nil, representing the final exit state.  state functions return
// the next state function.
func (l *lexer) run() {
    for state := lexAtom; state != nil; {
        state = state(l)
    }
    close(l.items)
}

// emit a lexer item with the given type and the string representing
// the current region that was being lexed
func (l *lexer) emit(t itemType) {
    l.items <- item{t, l.input[l.start:l.pos]}
    l.start = l.pos
}

func emitHelper(l *lexer, t itemType, nextState stateFn) stateFn {
    if (l.pos > l.start) {
        l.emit(t)
    }
    return nextState
}

// state for lexing an atom
func lexAtom(l *lexer) stateFn {
    for {
        if l.peek() == '(' {
            return emitHelper(l, itemAtom, lexLeftParen)
        }
        if l.peek() == ')' {
            return emitHelper(l, itemAtom, lexRightParen)
        }
        if l.peek() == '"' {
            nextState := emitHelper(l, itemAtom, lexDQuote)
            l.next()
            return nextState
        }
        if l.peek() == ' '  || l.peek() == '\t' || 
           l.peek() == '\r' || l.peek() == '\n' {
            return emitHelper(l, itemAtom, lexWhitespace)
        }
        if l.next() == eof { break }
    }
    if l.pos > l.start {
        l.emit(itemAtom)
    }
    l.emit(itemEOF)
    return nil
}

// state for lexing a double quoted string
func lexDQuote(l *lexer) stateFn {
    if l.accept("\"") {
        l.emit(itemAtom)
        return lexAtom
    }
    l.next()
    return lexDQuote
}

// state to spin through whitespace and throw it out between atoms
func lexWhitespace(l *lexer) stateFn {
    whitespace := " \r\n\t"
    if l.accept(whitespace) {
        l.ignore()
        return lexWhitespace
    }
    return lexAtom
}

// state matching a left paren
func lexLeftParen(l *lexer) stateFn {
    l.pos += 1
    l.emit(itemLParen)
    return lexAtom
}

// state matching a right paren
func lexRightParen(l *lexer) stateFn {
    l.pos += 1
    l.emit(itemRParen)
    return lexAtom
}


// see if we can match the next item in the string to some element in the
// string provided
func (l *lexer) accept(valid string) bool {
    if strings.IndexRune(valid, l.next()) >= 0 {
        return true
    }
    l.backup()
    return false
}

// ignore the most recent character
func (l *lexer) ignore() {
    l.start = l.pos
}

// back up one
func (l *lexer) backup() {
    l.pos -= l.width
}

// peek ahead but don't advance the position
func (l *lexer ) peek() int {
    rune := l.next()
    l.backup()
    return rune
}

// advance the position (if we can) and return the rune that was consumed
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

// spin through a channel of items and print them out until we hit the EOF
// item
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
    testexpr := "(test (test2 \"i am long\" test3) blah)"
    l, items := lex("S-Expression Lexer",testexpr)
    s := parse(items)
    fmt.Println(l.name)
    //  printall(items)
    fmt.Println(s)
    fmt.Println("EXPR=",testexpr)
    sexprToDotFile(s,"test.dot");
}