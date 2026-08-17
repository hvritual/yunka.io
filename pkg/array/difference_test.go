package array

import "testing"

/**
* @Description: TODO
* @date 2019-09-22
* @version V1.0
 */
func TestStringDifference(t *testing.T) {
	t.Log(StringDifference([]string{
		"1",
		"2",
		"3",
		"Biology",
		"test",
		"4",
		"5",
		"test122",
	}, []string{
		"Biology",
		"4",
		"5",
	}))

	t.Log(StringDifference([]string{
		"Biology",
		"4",
		"5",
	}, []string{
		"1",
		"2",
		"3",
		"Biology",
		"test",
		"4",
		"5",
		"test122",
	}))

	t.Log(StringDifference([]string{
		"1",
		"2",
		"3",
		"Biology",
		"test",
		"4ok",
		"5shhh",
		"test122",
	}, []string{
		"1",
		"2",
		"3",
		"Biology",
		"test",
		"4",
		"5",
		"test122",
	}))

	t.Log(StringDifference([]string{
		"1",
		"2",
		"3",
		"4",
		"5",
	}, []string{
		"5",
		"6",
	}))

	t.Log(StringDifference([]string{
		"1",
		"2",
		"3",
	}, []string{
		"3",
		"5",
		"6",
	}))

	p := []string{"1", "2", "3", "4", "5"}
	p1 := []string{"5", "6"}
	common := StringIntersect(p, p1)
	add := StringDifference(p, p1)
	delete := StringDifference(p1, common)
	t.Log(common, add, delete)

}
