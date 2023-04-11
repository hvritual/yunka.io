package array

import "sort"

/**
* @Description: TODO
* @date 2019-09-25
* @version V1.0
 */
func StringRemoveRepeated(arr []string) (newArr []string) {
	newArr = make([]string, 0)
	sort.Strings(arr)
	for i := 0; i < len(arr); i++ {
		repeat := false
		for j := i + 1; j < len(arr); j++ {
			if arr[i] == arr[j] {
				repeat = true
				break
			}
		}
		if !repeat {
			newArr = append(newArr, arr[i])
		}
	}
	return
}

func StringRemoveString(arr []string, index string) []string {
	for i := 0; i < len(arr); i++ {
		if arr[i] == index {
			if i == len(arr)-1 {
				return arr[:i]
			} else {
				return append(arr[:i], arr[i+1:]...)
			}
		}
	}
	return arr
}
