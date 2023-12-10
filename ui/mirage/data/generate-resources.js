/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function generateResources() {
  return {
    CpuStats: {
      Measured: ['Throttled Periods', 'Throttled Time', 'Percent'],
      Percent: 0.14159538847117795,
      SystemMode: 0,
      ThrottledPeriods: 0,
      ThrottledTime: 0,
      TotalTicks: 300.256693934837093,
      UserMode: 0,
    },
    MemoryStats: {
      Cache: 1744896,
      KernelMaxUsage: 0,
      KernelUsage: 0,
      MaxUsage: 4710400,
      Measured: ['RSS', 'Cache', 'Swap', 'Max Usage'],
      RSS: 1486848009,
      Swap: 0,
    },
  };
}
