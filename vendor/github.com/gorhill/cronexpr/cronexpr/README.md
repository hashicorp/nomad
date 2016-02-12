cronexpr: command-line utility
==============================

A command-line utility written in Go to evaluate cron time expressions.

It is based on the standalone Go library <https://github.com/gorhill/cronexpr>.

## Install

    go get github.com/gorhill/cronexpr
    go install github.com/gorhill/cronexpr

## Usage

    cronexpr [options] "{cron expression}"

## Options

`-l`:

Go-compliant time layout to use for outputting time value(s), see <http://golang.org/pkg/time/#pkg-constants>.

Default is `"Mon, 02 Jan 2006 15:04:05 MST"`

`-n`:

Number of resulting time values to output.

Default is 1.

`-t`:

Whole or partial RFC3339 time value (i.e. `2006-01-02T15:04:05Z07:00`) against which the cron expression is evaluated. Examples of valid values include (assuming EST time zone):

`13` = 2013-01-01T00:00:00-05:00  
`2013` = 2013-01-01T00:00:00-05:00  
`2013-08` = 2013-08-01T00:00:00-05:00  
`2013-08-31` = 2013-08-31T00:00:00-05:00  
`2013-08-31T12` = 2013-08-31T12:00:00-05:00  
`2013-08-31T12:40` = 2013-08-31T12:40:00-05:00  
`2013-08-31T12:40:35` = 2013-08-31T12:40:35-05:00  
`2013-08-31T12:40:35-10:00` = 2013-08-31T12:40:35-10:00  

Default time is current time, and default time zone is local time zone.

## Examples

#### Example 1

Midnight on December 31st of any year.

Command:

    cronexpr -t="2013-08-31" -n=5 "0 0 31 12 *"

Output (assuming computer is in EST time zone):

    # "0 0 31 12 *" + "2013-08-31T00:00:00-04:00" =
    Tue, 31 Dec 2013 00:00:00 EST
    Wed, 31 Dec 2014 00:00:00 EST
    Thu, 31 Dec 2015 00:00:00 EST
    Sat, 31 Dec 2016 00:00:00 EST
    Sun, 31 Dec 2017 00:00:00 EST

#### Example 2

2pm on February 29th of any year.

Command:

    cronexpr -t=2000 -n=10 "0 14 29 2 *"

Output (assuming computer is in EST time zone):

    # "0 14 29 2 *" + "2000-01-01T00:00:00-05:00" =
    Tue, 29 Feb 2000 14:00:00 EST
    Sun, 29 Feb 2004 14:00:00 EST
    Fri, 29 Feb 2008 14:00:00 EST
    Wed, 29 Feb 2012 14:00:00 EST
    Mon, 29 Feb 2016 14:00:00 EST
    Sat, 29 Feb 2020 14:00:00 EST
    Thu, 29 Feb 2024 14:00:00 EST
    Tue, 29 Feb 2028 14:00:00 EST
    Sun, 29 Feb 2032 14:00:00 EST
    Fri, 29 Feb 2036 14:00:00 EST

#### Example 3

12pm on the work day closest to the 15th of March and every three month
thereafter.

Command:

    cronexpr -t=2013-09-01 -n=5 "0 12 15W 3/3 *"

Output (assuming computer is in EST time zone):

    # "0 12 15W 3/3 *" + "2013-09-01T00:00:00-04:00" =
    Mon, 16 Sep 2013 12:00:00 EDT
    Mon, 16 Dec 2013 12:00:00 EST
    Fri, 14 Mar 2014 12:00:00 EDT
    Mon, 16 Jun 2014 12:00:00 EDT
    Mon, 15 Sep 2014 12:00:00 EDT

#### Example 4

Midnight on the fifth Saturday of any month (twist: not all months have a 5th
specific day of week).

Command:

    cronexpr -t=2013-09-02 -n 5 "0 0 * * 6#5"

Output (assuming computer is in EST time zone):

    # "0 0 * * 6#5" + "2013-09-02T00:00:00-04:00" =
    Sat, 30 Nov 2013 00:00:00 EST
    Sat, 29 Mar 2014 00:00:00 EDT
    Sat, 31 May 2014 00:00:00 EDT
    Sat, 30 Aug 2014 00:00:00 EDT
    Sat, 29 Nov 2014 00:00:00 EST

