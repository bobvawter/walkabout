// Code generated by "stringer -type Kind"; DO NOT EDIT.

package ts

import "strconv"

const _Kind_name = "InterfaceSliceStructPointer"

var _Kind_index = [...]uint8{0, 9, 14, 20, 27}

func (i Kind) String() string {
	i -= 1
	if i < 0 || i >= Kind(len(_Kind_index)-1) {
		return "Kind(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _Kind_name[_Kind_index[i]:_Kind_index[i+1]]
}
