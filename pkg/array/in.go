package array

/**
* @Description: TODO
* @date 2019-09-25
* @version V1.0
 */
func StringIn(array []string, index string) bool {
	return ContainsString(array, index) != -1
}

func ContainsString(array []string, val string) (index int) {
	index = -1
	for i := 0; i < len(array); i++ {
		if array[i] == val {
			index = i
			return
		}
	}
	return
}
