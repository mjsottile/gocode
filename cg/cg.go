package main

import "fmt"
import "container/vector"

//
// category game model in go
//

type nodeType int

const (
  nodeRange nodeType = iota
  nodeCategory
)

type RangeTree struct {
  names vector.IntVector
  typ nodeType
  lo float32
  hi float32
  ctr float32
  left, right *RangeTree
}

func (r RangeTree) String() string {
	s := fmt.Sprintf("(%s %d %f/%f/%f %s %s)",
		r.names,
		r.typ,
		r.lo,
		r.ctr,
		r.hi,
		r.left,
		r.right)
	return s
}

func newRangeTree (lo float32, hi float32) (*RangeTree) {
  r := &RangeTree {
	names : make(vector.IntVector, 0),
        lo : lo,
        hi : hi,
        ctr : (hi+lo)/2,
        left : nil,
        right : nil,
        typ : nodeCategory,
  }
  return r
}

func splitCategoryAt (namegen chan int, dict *RangeTree, splt float32) {
  if (dict == nil) {
    return
  }
  switch dict.typ {
	case nodeCategory:
		lname := <-namegen
		rname := <-namegen
		lcat := newRangeTree (dict.lo, splt)
		rcat := newRangeTree (splt, dict.hi)
		lcat.names = dict.names.Copy()
		rcat.names = dict.names.Copy()
		lcat.names.Push(lname)
		rcat.names.Push(rname)
		dict.left = lcat
		dict.right = rcat
		dict.typ = nodeRange
	case nodeRange:
		if (dict.ctr > splt) {
			splitCategoryAt(namegen, dict.left, splt)
                } else {
			splitCategoryAt(namegen, dict.right, splt)
                }
	default:
		panic("oh no!")
  }
}

func namegenerator () (chan int) {
  ch := make(chan int)
  f := func(ch chan int) {
    for i := 0; ;i++ { ch <- i }
  }
  go f(ch)
  return ch
}

func main() {
  c := namegenerator()

  rt := newRangeTree(0,1)

  fmt.Println(rt)

  splitCategoryAt(c, rt, 0.25)
  fmt.Println(rt)

  splitCategoryAt(c, rt, 0.75)
  fmt.Println(rt)

  splitCategoryAt(c, rt, 0.85)
  fmt.Println(rt)

}
