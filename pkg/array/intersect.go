package array

import "sort"

/**
* @Description: TODO
* @date 2019-09-22
* @version V1.0
 */

func StringIntersect(a1, a2 []string) []string {
	var intersect []string
	sort.Strings(a1)
	sort.Strings(a2)

	i, j := 0, 0
	for i < len(a1) && j < len(a2) {
		if a1[i] == a2[j] {
			intersect = append(intersect, a1[i])
			i++
			j++
		} else if a1[i] < a2[j] {
			i++
		} else {
			j++
		}
	}
	return intersect
}
