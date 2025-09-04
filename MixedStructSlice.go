package mss

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"unsafe"
)

type runtimeSlice struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

type iface struct{ tab, data unsafe.Pointer } // non-empty interface layout
type eface struct{ typ, data unsafe.Pointer }

type MixedStructSlice struct {
	types      []reflect.Type
	sliceType  reflect.Type
	slice      reflect.Value
	sliceIface any
	slicertptr unsafe.Pointer

	sh      *runtimeSlice
	offsets []uintptr
	rtptrs  []unsafe.Pointer
	stride  uintptr
}

type RowViewMut struct {
	mss *MixedStructSlice
	row int
}

type RowViewCopy struct {
	mss    *MixedStructSlice
	row    int
	bufrow int
	buffer []uintptr
}

func (mss *MixedStructSlice) initHeadersFromSlice(s reflect.Value) {
	mss.slice = s
	mss.sliceIface = s.Interface()
	mss.sh = (*runtimeSlice)((*eface)(unsafe.Pointer(&mss.sliceIface)).data)
}

func (mss *MixedStructSlice) ensure(extra int) {
	need := mss.slice.Len() + extra
	if mss.slice.Cap() >= need {
		return
	}
	newCap := nextslicecap(mss.slice.Len()+extra, mss.slice.Cap())
	ns := reflect.MakeSlice(reflect.SliceOf(mss.sliceType), mss.slice.Len(), newCap)
	reflect.Copy(ns, mss.slice)
	mss.initHeadersFromSlice(ns)
}
func (mss *MixedStructSlice) pushRow() (row int, base unsafe.Pointer) {
	mss.ensure(1)
	row = mss.sh.Len
	mss.sh.Len = row + 1

	base = unsafe.Add(mss.sh.Data, uintptr(row)*mss.stride)
	runtime.KeepAlive(mss.slice)
	return
}

// Adds types of structs to store
func (mss *MixedStructSlice) AddComponent(comp any) *MixedStructSlice {
	mss.types = append(mss.types, reflect.TypeOf(comp))
	return mss
}

// Build must be called after all desired AddComponent() calls
// Build must be called before any Add() calls
func (mss *MixedStructSlice) Build() {
	newTypeFields := []reflect.StructField{}
	for _, tt := range mss.types {
		newTypeFields = append(newTypeFields, reflect.StructField{
			Name: "D" + strconv.Itoa(len(newTypeFields)),
			//Pkgpath MUST be empty to export(decode) the fields later
			PkgPath: "",
			Type:    tt,
		})
	}
	mss.sliceType = reflect.StructOf(newTypeFields)
	s := reflect.MakeSlice(reflect.SliceOf(mss.sliceType), 0, 1)
	mss.initHeadersFromSlice(s)
	mss.stride = uintptr(mss.sliceType.Size())
	for i := range mss.sliceType.NumField() {
		mss.offsets = append(mss.offsets, uintptr(mss.sliceType.Field(i).Offset))
		mss.rtptrs = append(mss.rtptrs, rtypePtr(mss.sliceType.Field(i).Type))
	}
	mss.slicertptr = rtypePtr(mss.sliceType)
}

// Adds a row to the slice
// Must be called after Build()
// Must be given full rows, and in order of how they were added with AddComponent()
func (mss *MixedStructSlice) Add(comps ...any) int {
	if len(comps) != len(mss.types) {
		panic("number of types given to AOS differs from AOS size")
	}
	//aos.slice = reflect.Append(aos.slice, reflect.Zero(aos.sliceType))
	row, base := mss.pushRow()

	for col, comp := range comps {
		dst := unsafe.Add(base, mss.offsets[col])
		src := dataPtr(comp)
		typedmemmove(mss.rtptrs[col], dst, src)
	}
	runtime.KeepAlive(mss.slice)
	return row
}

// Returns length of the slice
func (mss *MixedStructSlice) Len() int {
	return mss.sh.Len
}

// Removes a row from the given slice
func (mss *MixedStructSlice) SwapDelete(row int) {
	if row < 0 || row >= mss.Len() {
		panic(fmt.Errorf("slice index %d out of bounds, max: %d", row, mss.Len()))
	}
	src := unsafe.Add(mss.sh.Data, mss.stride*uintptr(mss.Len()-1))
	dst := unsafe.Add(mss.sh.Data, mss.stride*uintptr(row))
	typedmemmove(mss.slicertptr, dst, src)
	mss.sh.Len--
}

func NewRowViewMut(mss *MixedStructSlice) RowViewMut {
	return RowViewMut{
		mss: mss,
		row: 0,
	}
}

func (rv *RowViewMut) SetIndex(i int) {
	if i < 0 || i >= rv.mss.Len() {
		panic(fmt.Errorf("index out of range %d >= %d", i, rv.mss.Len()))
	}
	rv.row = i
}

func (rv *RowViewMut) Next() {
	if rv.row < rv.mss.Len()-1 {
		rv.row++
	}
}

func NewRowViewCopy(mss *MixedStructSlice) RowViewCopy {
	const ps = unsafe.Sizeof(uintptr(0))
	return RowViewCopy{
		mss:    mss,
		row:    0,
		bufrow: -1,
		buffer: make([]uintptr, (mss.stride+ps-1)/ps),
	}
}

func (rv *RowViewCopy) SetIndex(i int) {
	if i < 0 || i >= rv.mss.Len() {
		panic(fmt.Errorf("index out of range %d >= %d", i, rv.mss.Len()))
	}
	rv.row = i
}

func (rv *RowViewCopy) Next() {
	if rv.row < rv.mss.Len()-1 {
		rv.row++
	}
}

func RowGet[T any](r RowViewMut, col int) *T {
	return IndexRowCol[T](r.mss, r.row, col)
}

func RowGetCopy[T any](r RowViewCopy, col int) T {
	tt := *((*T)(unsafe.Add(r.mss.sh.Data, r.mss.stride*uintptr(r.row)+r.mss.offsets[col])))
	return tt
}

// Returns the column associated with the given type in the slice
func ColOf[storedType any](mss *MixedStructSlice) int {
	var val storedType
	compType := reflect.TypeOf(val)
	for v, tt := range mss.types {
		if tt == compType {
			return v
		}
	}
	return -1
}

// Returns the struct of type storedType stored at index i
func Index[storedType any](mss *MixedStructSlice, i int) (val *storedType) {
	compType := reflect.TypeOf(val).Elem()
	index := -1
	for v, tt := range mss.types {
		if tt == compType {
			index = v
			break
		}
	}
	val = IndexRowCol[storedType](mss, i, index)
	return
}

// Returns the object stored at row r, and column c
// This function does NOT check if the right type is stored at r,c
func IndexRowCol[storedType any](mss *MixedStructSlice, r, c int) (val *storedType) {
	if r < 0 || r >= mss.Len() || c < 0 || c >= len(mss.types) {
		if r > 0 || r >= mss.Len() {
			panic(fmt.Errorf("row %d out of range, max: %d", r, mss.Len()-1))
		}
		if c < 0 || c >= len(mss.types) {
			panic(fmt.Errorf("column %d out of range, max: %d", c, len(mss.types)-1))
		}
	}
	return (*storedType)(unsafe.Add(mss.sh.Data, mss.stride*uintptr(r)+mss.offsets[c]))
}

// Taken directly from golang source runtime/slice.go
func nextslicecap(newLen, oldCap int) int {
	newcap := oldCap
	doublecap := newcap + newcap
	if newLen > doublecap {
		return newLen
	}

	const threshold = 256
	if oldCap < threshold {
		return doublecap
	}
	for {
		// Transition from growing 2x for small slices
		// to growing 1.25x for large slices. This formula
		// gives a smooth-ish transition between the two.
		newcap += (newcap + 3*threshold) >> 2

		// We need to check `newcap >= newLen` and whether `newcap` overflowed.
		// newLen is guaranteed to be larger than zero, hence
		// when newcap overflows then `uint(newcap) > uint(newLen)`.
		// This allows to check for both with the same comparison.
		if uint(newcap) >= uint(newLen) {
			break
		}
	}

	// Set newcap to the requested cap when
	// the newcap calculation overflowed.
	if newcap <= 0 {
		return newLen
	}
	return newcap
}

func rtypePtr(t reflect.Type) unsafe.Pointer {
	// reflect.Type is an interface whose concrete type is *rtype.
	// The data word points at the rtype header, which is ABI-compatible with abi.Type.
	return (*iface)(unsafe.Pointer(&t)).data
}

func dataPtr(x any) unsafe.Pointer {
	return (*iface)(unsafe.Pointer(&x)).data
}

// Might be able to use normal memmove?
//
//go:linkname typedmemmove runtime.typedmemmove
func typedmemmove(typ unsafe.Pointer, dst, src unsafe.Pointer)
