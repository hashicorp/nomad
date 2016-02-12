/*!
 * Copyright 2013 Raymond Hill
 *
 * Project: github.com/gorhill/cronexpr
 * File: main.go
 * Version: 1.0
 * License: GPL v3 see <https://www.gnu.org/licenses/gpl.html>
 *
 */

package main

/******************************************************************************/

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gorhill/cronexpr"
)

/******************************************************************************/

var (
	usage = func() {
		fmt.Fprintf(os.Stderr, "usage:\n  %s [options] \"{cron expression}\"\noptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	inTimeStr     string
	outTimeCount  uint
	outTimeLayout string
)

/******************************************************************************/

func main() {
	var err error

	flag.Usage = usage
	flag.StringVar(&inTimeStr, "t", "", `whole or partial RFC3339 time value (i.e. "2006-01-02T15:04:05Z07:00") against which the cron expression is evaluated, now if not present`)
	flag.UintVar(&outTimeCount, "n", 1, `number of resulting time values to output`)
	flag.StringVar(&outTimeLayout, "l", "Mon, 02 Jan 2006 15:04:05 MST", `Go-compliant time layout to use for outputting time value(s), see <http://golang.org/pkg/time/#pkg-constants>`)
	flag.Parse()

	cronStr := flag.Arg(0)
	if len(cronStr) == 0 {
		flag.Usage()
		return
	}

	inTime := time.Now()
	inTimeLayout := ""
	timeStrLen := len(inTimeStr)
	if timeStrLen == 2 {
		inTimeLayout = "06"
	} else if timeStrLen >= 4 {
		inTimeLayout += "2006"
		if timeStrLen >= 7 {
			inTimeLayout += "-01"
			if timeStrLen >= 10 {
				inTimeLayout += "-02"
				if timeStrLen >= 13 {
					inTimeLayout += "T15"
					if timeStrLen >= 16 {
						inTimeLayout += ":04"
						if timeStrLen >= 19 {
							inTimeLayout += ":05"
							if timeStrLen >= 20 {
								inTimeLayout += "Z07:00"
							}
						}
					}
				}
			}
		}
	}

	if len(inTimeLayout) > 0 {
		// default to local time zone
		if timeStrLen < 20 {
			inTime, err = time.ParseInLocation(inTimeLayout, inTimeStr, time.Local)
		} else {
			inTime, err = time.Parse(inTimeLayout, inTimeStr)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "# error: unparseable time value: \"%s\"\n", inTimeStr)
			os.Exit(1)
		}
	}

	expr, err := cronexpr.Parse(cronStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "# %s: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	// Anything on the output which starts with '#' can be ignored if the caller
	// is interested only in the time values. There is only one time
	// value per line, and they are always in chronological ascending order.
	fmt.Printf("# \"%s\" + \"%s\" =\n", cronStr, inTime.Format(time.RFC3339))

	if outTimeCount < 1 {
		outTimeCount = 1
	}
	outTimes := expr.NextN(inTime, outTimeCount)
	for _, outTime := range outTimes {
		fmt.Println(outTime.Format(outTimeLayout))
	}
}
