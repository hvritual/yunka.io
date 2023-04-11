package array

import (
	"testing"
)

/**
* @Description: TODO
* @date 2019-09-22
* @version V1.0
 */
func TestStringIntersect(t *testing.T) {
	t.Log(StringIntersect([]string{
		"Cooking",
		"English",
		"Math",
		"Math",
		"Biology",
	}, []string{
		"Biology",
		"Chemistry",
	}))

	t.Log(StringIntersect([]string{
		"Cooking",
		"English",
		"Math",
		"Math",
		"Biology",
	}, []string{
		"Chemistry",
	}))

	t.Log(StringIntersect([]string{
		"Cooking",
		"English",
		"Math",
		"Math",
		"Biology",
	}, nil))

	t.Log(StringIntersect(nil, []string{
		"Chemistry",
	}))
}
