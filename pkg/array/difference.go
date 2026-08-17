package array

/**
* @Description: TODO
* @date 2019-09-22
* @version V1.0
 */
func StringDifference(a1, a2 []string) []string {
	var diff []string
	lenA2 := len(a2)
	lenA1 := len(a1)
	if lenA2 == 0 {
		return a1
	}

	if lenA1 > lenA2 {
		for i := 0; i < lenA1; i++ {
			if index := ContainsString(a2, a1[i]); index == -1 {
				diff = append(diff, a1[i])
			}
		}
	} else {
		for i := 0; i < lenA2; i++ {
			if index := ContainsString(a1, a2[i]); index == -1 {
				diff = append(diff, a2[i])
			}
		}
	}


	return diff
}
