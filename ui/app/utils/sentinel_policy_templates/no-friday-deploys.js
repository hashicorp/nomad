/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default `import "time"

is_weekday = rule { time.day not in ["friday", "saturday", "sunday"] }
is_open_hours = rule { time.hour > 8 and time.hour < 16 }

main = rule { is_open_hours and is_weekday }`;
