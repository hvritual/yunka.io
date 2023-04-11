package array

/**
* @Description: TODO
* @date 2019-09-22
* @version V1.0
 */
func StringUnion(a1, a2 []string) []string {
	a1 = append(a1, a2...)
	return StringRemoveRepeated(a1)
}
