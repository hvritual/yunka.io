package array

import (
	"github.com/pkg/errors"
	"math/rand"
	"time"
)

/**
 * @BelongProject hub
 * @BelongPackage array
 * @Description:
 *
 * @Copyright 2020 5pluscloud - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/2/19 12:13 下午
 * @Version V1.0
 */

func init() {
	rand.Seed(time.Now().Unix())
}

func Random(strings []string) ([]string, error) {
	if len(strings) <= 0 {
		return nil, errors.New("the length of the parameter strings should not be less than 0")
	}

	for i := len(strings) - 1; i > 0; i-- {
		num := rand.Intn(i + 1)
		strings[i], strings[num] = strings[num], strings[i]
	}

	return strings, nil
}
