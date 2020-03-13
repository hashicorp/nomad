/*!
 * Copyright 2013 Raymond Hill
 *
 * Project: github.com/gorhill/cronexpr
 * File: cronexpr_next.go
 * Version: 1.0
 * License: pick the one which suits you :
 *   GPL v3 see <https://www.gnu.org/licenses/gpl.html>
 *   APL v2 see <http://www.apache.org/licenses/LICENSE-2.0>
 *
 */

package cronexpr

/******************************************************************************/

import (
	"sort"
	"time"
)

/******************************************************************************/

var dowNormalizedOffsets = [][]int{
	{1, 8, 15, 22, 29},
	{2, 9, 16, 23, 30},
	{3, 10, 17, 24, 31},
	{4, 11, 18, 25},
	{5, 12, 19, 26},
	{6, 13, 20, 27},
	{7, 14, 21, 28},
}

/******************************************************************************/

func (expr *Expression) nextYear(t time.Time) time.Time {
	// Find index at which item in list is greater or equal to
	// candidate year
	i := sort.SearchInts(expr.yearList, t.Year()+1)
	if i == len(expr.yearList) {
		return time.Time{}
	}
	// Year changed, need to recalculate actual days of month
	expr.actualDaysOfMonthList = expr.calculateActualDaysOfMonth(expr.yearList[i], expr.monthList[0])
	if len(expr.actualDaysOfMonthList) == 0 {
		return expr.nextMonth(time.Date(
			expr.yearList[i],
			time.Month(expr.monthList[0]),
			1,
			expr.hourList[0],
			expr.minuteList[0],
			expr.secondList[0],
			0,
			t.Location()))
	}

	next := time.Date(
		expr.yearList[i],
		time.Month(expr.monthList[0]),
		expr.actualDaysOfMonthList[0],
		expr.hourList[0],
		expr.minuteList[0],
		expr.secondList[0],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

/******************************************************************************/

func (expr *Expression) nextMonth(t time.Time) time.Time {
	// Find index at which item in list is greater or equal to
	// candidate month
	i := sort.SearchInts(expr.monthList, int(t.Month())+1)
	if i == len(expr.monthList) {
		return expr.nextYear(t)
	}
	// Month changed, need to recalculate actual days of month
	expr.actualDaysOfMonthList = expr.calculateActualDaysOfMonth(t.Year(), expr.monthList[i])
	if len(expr.actualDaysOfMonthList) == 0 {
		return expr.nextMonth(time.Date(
			t.Year(),
			time.Month(expr.monthList[i]),
			1,
			expr.hourList[0],
			expr.minuteList[0],
			expr.secondList[0],
			0,
			t.Location()))
	}

	next := time.Date(
		t.Year(),
		time.Month(expr.monthList[i]),
		expr.actualDaysOfMonthList[0],
		expr.hourList[0],
		expr.minuteList[0],
		expr.secondList[0],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

/******************************************************************************/

func (expr *Expression) nextDayOfMonth(t time.Time) time.Time {
	// Find index at which item in list is greater or equal to
	// candidate day of month
	i := sort.SearchInts(expr.actualDaysOfMonthList, t.Day()+1)
	if i == len(expr.actualDaysOfMonthList) {
		return expr.nextMonth(t)
	}

	next := time.Date(
		t.Year(),
		t.Month(),
		expr.actualDaysOfMonthList[i],
		expr.hourList[0],
		expr.minuteList[0],
		expr.secondList[0],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

/******************************************************************************/

func (expr *Expression) nextHour(t time.Time) time.Time {
	// Find index at which item in list is greater or equal to
	// candidate hour
	i := sort.SearchInts(expr.hourList, t.Hour()+1)
	if i == len(expr.hourList) {
		return expr.nextDayOfMonth(t)
	}

	next := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		expr.hourList[i],
		expr.minuteList[0],
		expr.secondList[0],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

/******************************************************************************/

func (expr *Expression) nextMinute(t time.Time) time.Time {
	// Find index at which item in list is greater or equal to
	// candidate minute
	i := sort.SearchInts(expr.minuteList, t.Minute()+1)
	if i == len(expr.minuteList) {
		return expr.nextHour(t)
	}

	next := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		expr.minuteList[i],
		expr.secondList[0],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

/******************************************************************************/

func (expr *Expression) nextSecond(t time.Time) time.Time {
	// nextSecond() assumes all other fields are exactly matched
	// to the cron expression

	// Find index at which item in list is greater or equal to
	// candidate second
	i := sort.SearchInts(expr.secondList, t.Second()+1)
	if i == len(expr.secondList) {
		return expr.nextMinute(t)
	}

	next := time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		expr.secondList[i],
		0,
		time.UTC)

	return expr.nextTime(t, next)
}

func (expr *Expression) roundTime(t time.Time) time.Time {
	roundingExpr := new(Expression)
	*roundingExpr = *expr
	roundingExpr.rounding = true

	i := sort.SearchInts(expr.hourList, t.Hour())
	if i == len(expr.hourList) || expr.hourList[i] != t.Hour() {
		return roundingExpr.nextHour(t)
	}

	i = sort.SearchInts(expr.minuteList, t.Minute())
	if i == len(expr.minuteList) || expr.minuteList[i] != t.Minute() {
		return roundingExpr.nextMinute(t)
	}

	i = sort.SearchInts(expr.secondList, t.Second())
	if i == len(expr.secondList) || expr.secondList[i] != t.Second() {
		return roundingExpr.nextSecond(t)
	}

	return t
}

func (expr *Expression) isRounded(t time.Time) bool {
	i := sort.SearchInts(expr.hourList, t.Hour())
	if i == len(expr.hourList) || expr.hourList[i] != t.Hour() {
		return false
	}

	i = sort.SearchInts(expr.minuteList, t.Minute())
	if i == len(expr.minuteList) || expr.minuteList[i] != t.Minute() {
		return false
	}

	i = sort.SearchInts(expr.secondList, t.Second())
	if i == len(expr.secondList) || expr.secondList[i] != t.Second() {
		return false
	}

	return true
}

func (expr *Expression) nextTime(prev, next time.Time) time.Time {
	dstFlags := expr.options.DSTFlags
	t := prev.Add(noTZDiff(prev, next))
	offsetDiff := utcOffset(t) - utcOffset(prev)

	// a dst leap occurred
	if offsetDiff > 0 {
		dstChangeTime := findTimeOfDSTChange(prev, t)

		// since a dst leap occured, t is offsetDiff seconds ahead
		t = t.Add(-1 * offsetDiff)

		// check if t is within the skipped interval (offsetDiff)
		if noTZDiff(dstChangeTime, t) < offsetDiff {
			if dstFlags&DSTLeapUnskip != 0 {
				// return the earliest time right after the leap
				return dstChangeTime.Add(1 * time.Second)
			}

			// return the next scheduled time right after the leap
			return expr.roundTime(dstChangeTime.Add(1 * time.Second))
		}

		return t
	}

	// a dst fall occurred
	if offsetDiff < 0 {
		twinT := findTwinTime(prev)

		if !twinT.IsZero() {
			if dstFlags&DSTFallFireLate != 0 {
				return twinT
			}
			if dstFlags&DSTFallFireEarly != 0 {
				// skip the twin time
				return expr.Next(expr.roundTime(twinT))
			}
		}

		if dstFlags&DSTFallFireEarly != 0 {
			return t
		}

		return expr.roundTime(t)
	}

	twinT := findTwinTime(t)
	if !twinT.IsZero() {
		if dstFlags&DSTFallFireEarly != 0 {
			return t
		}
		if dstFlags&DSTFallFireLate != 0 {
			return twinT
		}
	}

	if dstFlags&DSTFallFireLate == 0 && !expr.rounding {
		twinT = findTwinTime(prev)
		if !twinT.IsZero() && twinT.Before(prev) && !expr.isRounded(prev) {
			return expr.Next(t)
		}
	}

	return t
}

/******************************************************************************/

func (expr *Expression) calculateActualDaysOfMonth(year, month int) []int {
	actualDaysOfMonthMap := make(map[int]bool)
	firstDayOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	lastDayOfMonth := firstDayOfMonth.AddDate(0, 1, -1)

	// As per crontab man page (http://linux.die.net/man/5/crontab#):
	//  "The day of a command's execution can be specified by two
	//  "fields - day of month, and day of week. If both fields are
	//  "restricted (ie, aren't *), the command will be run when
	//  "either field matches the current time"

	// If both fields are not restricted, all days of the month are a hit
	if expr.daysOfMonthRestricted == false && expr.daysOfWeekRestricted == false {
		return genericDefaultList[1 : lastDayOfMonth.Day()+1]
	}

	// day-of-month != `*`
	if expr.daysOfMonthRestricted {
		// Last day of month
		if expr.lastDayOfMonth {
			actualDaysOfMonthMap[lastDayOfMonth.Day()] = true
		}
		// Last work day of month
		if expr.lastWorkdayOfMonth {
			actualDaysOfMonthMap[workdayOfMonth(lastDayOfMonth, lastDayOfMonth)] = true
		}
		// Days of month
		for v := range expr.daysOfMonth {
			// Ignore days beyond end of month
			if v <= lastDayOfMonth.Day() {
				actualDaysOfMonthMap[v] = true
			}
		}
		// Work days of month
		// As per Wikipedia: month boundaries are not crossed.
		for v := range expr.workdaysOfMonth {
			// Ignore days beyond end of month
			if v <= lastDayOfMonth.Day() {
				actualDaysOfMonthMap[workdayOfMonth(firstDayOfMonth.AddDate(0, 0, v-1), lastDayOfMonth)] = true
			}
		}
	}

	// day-of-week != `*`
	if expr.daysOfWeekRestricted {
		// How far first sunday is from first day of month
		offset := 7 - int(firstDayOfMonth.Weekday())
		// days of week
		//  offset : (7 - day_of_week_of_1st_day_of_month)
		//  target : 1 + (7 * week_of_month) + (offset + day_of_week) % 7
		for v := range expr.daysOfWeek {
			w := dowNormalizedOffsets[(offset+v)%7]
			actualDaysOfMonthMap[w[0]] = true
			actualDaysOfMonthMap[w[1]] = true
			actualDaysOfMonthMap[w[2]] = true
			actualDaysOfMonthMap[w[3]] = true
			if len(w) > 4 && w[4] <= lastDayOfMonth.Day() {
				actualDaysOfMonthMap[w[4]] = true
			}
		}
		// days of week of specific week in the month
		//  offset : (7 - day_of_week_of_1st_day_of_month)
		//  target : 1 + (7 * week_of_month) + (offset + day_of_week) % 7
		for v := range expr.specificWeekDaysOfWeek {
			v = 1 + 7*(v/7) + (offset+v)%7
			if v <= lastDayOfMonth.Day() {
				actualDaysOfMonthMap[v] = true
			}
		}
		// Last days of week of the month
		lastWeekOrigin := firstDayOfMonth.AddDate(0, 1, -7)
		offset = 7 - int(lastWeekOrigin.Weekday())
		for v := range expr.lastWeekDaysOfWeek {
			v = lastWeekOrigin.Day() + (offset+v)%7
			if v <= lastDayOfMonth.Day() {
				actualDaysOfMonthMap[v] = true
			}
		}
	}

	return toList(actualDaysOfMonthMap)
}

func workdayOfMonth(targetDom, lastDom time.Time) int {
	// If saturday, then friday
	// If sunday, then monday
	dom := targetDom.Day()
	dow := targetDom.Weekday()
	if dow == time.Saturday {
		if dom > 1 {
			dom -= 1
		} else {
			dom += 2
		}
	} else if dow == time.Sunday {
		if dom < lastDom.Day() {
			dom += 1
		} else {
			dom -= 2
		}
	}
	return dom
}

func utcOffset(t time.Time) time.Duration {
	_, offset := t.Zone()
	return time.Duration(offset) * time.Second
}

func noTZ(t time.Time) time.Time {
	return t.UTC().Add(utcOffset(t))
}

func noTZDiff(t1, t2 time.Time) time.Duration {
	t1 = noTZ(t1)
	t2 = noTZ(t2)
	return t2.Sub(t1)
}

// findTimeOfDSTChange returns the time a second before a DST change occurs,
// and returns zero time in case there's no DST change.
func findTimeOfDSTChange(t1, t2 time.Time) time.Time {
	if t1.Location() != t2.Location() || utcOffset(t1) == utcOffset(t2) || t1.Location() == time.UTC {
		return time.Time{}
	}

	// make sure t2 > t1
	if t2.Before(t1) {
		t := t2
		t1 = t2
		t2 = t
	}

	// do a binary search to find the time one second before the dst change
	len := t2.Unix() - t1.Unix()
	var a int64
	b := len
	for len > 1 {
		len = (b - a + 1) / 2
		if utcOffset(t1.Add(time.Duration(a+len)*time.Second)) != utcOffset(t1) {
			b = a + len
		} else {
			a = a + len
		}
	}

	return t1.Add(time.Duration(a) * time.Second)
}

// When a DST fall accurs, a certain interval of time is repeated. Once
// in DST time and once in standard time.
// findTwinTime tries to find the repated "twin" time if one exists.
func findTwinTime(t time.Time) time.Time {
	offsetDiff := utcOffset(t.Add(12*time.Hour)) - utcOffset(t)
	// a fall occurs within the next 12 hours
	if offsetDiff < 0 {
		border := findTimeOfDSTChange(t, t.Add(12*time.Hour))
		t0 := border.Add(offsetDiff)

		if t0.After(t) {
			return t
		}

		dur := t.Sub(t0)
		return border.Add(dur)
	}

	offsetDiff = utcOffset(t) - utcOffset(t.Add(-12*time.Second))
	// a fall occurred in the past 12 hours
	if offsetDiff < 0 {
		border := findTimeOfDSTChange(t.Add(-12*time.Hour), t)
		t0 := border.Add(offsetDiff)

		if t0.Add(-2 * offsetDiff).Before(t) {
			return t
		}

		dur := t.Sub(border)
		return t0.Add(dur)
	}

	return time.Time{}
}
