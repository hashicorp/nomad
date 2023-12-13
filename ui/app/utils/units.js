/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export const BYTES_UNITS = ['Bytes', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
export const HERTZ_UNITS = ['Hz', 'KHz', 'MHz', 'GHz', 'THz', 'PHz'];

const locale = window.navigator.locale || 'en';
const decimalFormatter = new Intl.NumberFormat(locale, {
  maximumFractionDigits: 2,
});
const roundFormatter = new Intl.NumberFormat(locale, {
  maximumFractionDigits: 0,
});

const unitReducer = (number = 0, interval, units, maxUnit, startingUnit) => {
  if (startingUnit && units.indexOf(startingUnit) !== -1) {
    units = units.slice(units.indexOf(startingUnit));
  }
  if (maxUnit && units.indexOf(maxUnit) !== -1) {
    units = units.slice(0, units.indexOf(maxUnit) + 1);
  }

  // Reduce negative numbers by temporarily flipping them positive.
  const negative = number < 0;
  if (negative) {
    number *= -1;
  }

  let unitIndex = 0;
  while (number >= interval && unitIndex < units.length - 1) {
    number /= interval;
    unitIndex++;
  }

  if (negative) {
    number *= -1;
  }
  return [number, units[unitIndex]];
};

export function reduceBytes(bytes = 0, maxUnitSize, startingUnitSize) {
  return unitReducer(bytes, 1024, BYTES_UNITS, maxUnitSize, startingUnitSize);
}

export function reduceHertz(hertz = 0, maxUnitSize, startingUnitSize) {
  return unitReducer(hertz, 1000, HERTZ_UNITS, maxUnitSize, startingUnitSize);
}

// General purpose formatters meant to reduce units as much
// as possible.
export function formatBytes(bytes, startingUnitSize) {
  const [number, unit] = reduceBytes(bytes, null, startingUnitSize);
  return `${decimalFormatter.format(number)} ${unit}`;
}

export function formatHertz(hertz, startingUnitSize) {
  const [number, unit] = reduceHertz(hertz, null, startingUnitSize);
  return `${decimalFormatter.format(number)} ${unit}`;
}

// Specialized formatters meant to reduce units to the resolution
// the scheduler and job specs operate at.
export function formatScheduledBytes(bytes, startingUnitSize) {
  const [number, unit] = reduceBytes(bytes, 'MiB', startingUnitSize);
  return `${roundFormatter.format(number)} ${unit}`;
}

export function formatScheduledHertz(hertz, startingUnitSize) {
  const [number, unit] = reduceHertz(hertz, 'MHz', startingUnitSize);
  return `${roundFormatter.format(number)} ${unit}`;
}

// Replaces the minus sign (−) with a hyphen (-).
// https://github.com/d3/d3-format/releases/tag/v2.0.0
export function replaceMinus(input) {
  return input.replace('−', '-');
}
