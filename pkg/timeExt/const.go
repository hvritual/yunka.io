package timeExt

import "time"

/**
 * @BelongProject yunka
 * @BelongPackage timeExt
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/27 9:40 下午
 * @Version V1.0
 */
const (
	OneYearTs = 365 * OneDayTs
	OneDayTs  = 24 * 3600
)

// 获取今天零点时间
func GetZeroTime(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// 获取特定时间结束时间
func GetFinishTime(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, d.Location())
}
