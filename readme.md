# Mixed Struct Slice
This module provides access to a simple implementation of a mixed struct slice dynamically. Allowing you to create a contigous slice of mixed structs without specifying the combined struct at compile time. This package makes heavy use of unsafe and is not suited for many applications. This module is also missing many needed features currently.

Overall the performance difference between this module and a real combined struct is ~2x slower. This can be somewhat avoided by returning the raw unsafe.Pointers instead of using generics to cast back to real types. 

## Example
The dynamic struct must first be built up by providing example structs.
```go

type myType struct {x int}
type myType2 struct {y float32,z int32}
// Currently mixedStructSlice cannot handle structs of single pointers
// Since they are stored as a special case
//type myType3 struct {w *int} 
type myType3 struct {w *int, w2 *int}

newSlice := mss.MixedStructSlice{}
//Slice must be given its struct types in order. 
newSlice.AddComponent(myType{})
newSlice.AddComponent(myType2{})
newSlice.AddComponent(myType3{})
//After the build call, no more structs can be added
newSlice.Build()

//New rows can be added by providing full rows
newSlice.Add(myType{10},myType2{20.10,10},myType3{})
//TODO: adding sparse rows

//Rows can be accessed by either providing just the row index, or both the row and column
_ = mss.Index[myType](newSlice,0)
_ = mss.IndexRowCol[myType](newSlice,0,0)

//TODO: deleting rows

//ColOf can be used to find the column of a given type
_ = ColOf[myType](newSlice)

//TODO: errorhandling and such
```