/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import moment from 'moment';
import { pluralize } from 'ember-inflector';

/**
 * Metadata for all unit types
 * name: identifier for the unit. Also maps to moment methods when applicable
 * suffix: the preferred suffix for a unit
 * inMoment: whether or not moment can be used to compute this unit value
 * pluralizable: whether or not this suffix can be pluralized
 * longSuffix: the suffix to use instead of suffix when longForm is true
 */
const allUnits = [
  { name: 'years', suffix: 'year', inMoment: true, pluralizable: true },
  { name: 'months', suffix: 'month', inMoment: true, pluralizable: true },
  { name: 'days', suffix: 'day', inMoment: true, pluralizable: true },
  {
    name: 'hours',
    suffix: 'h',
    longSuffix: 'hour',
    inMoment: true,
    pluralizable: false,
  },
  {
    name: 'minutes',
    suffix: 'm',
    longSuffix: 'minute',
    inMoment: true,
    pluralizable: false,
  },
  {
    name: 'seconds',
    suffix: 's',
    longSuffix: 'second',
    inMoment: true,
    pluralizable: false,
  },
  { name: 'milliseconds', suffix: 'ms', inMoment: true, pluralizable: false },
  { name: 'microseconds', suffix: 'µs', inMoment: false, pluralizable: false },
  { name: 'nanoseconds', suffix: 'ns', inMoment: false, pluralizable: false },
];

const pluralizeUnits = (amount, unit, longForm) => {
  let suffix;

  if (longForm && unit.longSuffix) {
    // Long form means always using full words (seconds insteand of s) which means
    // pluralization is necessary.
    suffix = amount === 1 ? unit.longSuffix : pluralize(unit.longSuffix);
  } else {
    // In the normal case, only pluralize based on the pluralizable flag
    suffix =
      amount === 1 || !unit.pluralizable ? unit.suffix : pluralize(unit.suffix);
  }

  // A space should go between the value and the unit when the unit is a full word
  // 300ns vs. 1 hour
  const addSpace = unit.pluralizable || (longForm && unit.longSuffix);
  return `${amount}${addSpace ? ' ' : ''}${suffix}`;
};

/**
 * Format a Duration at a preferred precision
 *
 * @param {Number} duration The duration to format
 * @param {String} units The units for the duration. Default to nanoseconds.
 * @param {Boolean} longForm Whether or not to expand single character suffixes,
 *   used to ensure screen readers correctly read units.
 */
export default function formatDuration(
  duration = 0,
  units = 'ns',
  longForm = false
) {
  const durationParts = {};

  // Moment only handles up to millisecond precision.
  // Microseconds and nanoseconds need to be handled first,
  // then Moment can take over for all larger units.
  if (units === 'ns') {
    durationParts.nanoseconds = duration % 1000;
    durationParts.microseconds = Math.floor((duration % 1000000) / 1000);
    duration = Math.floor(duration / 1000000);
  } else if (units === 'mms') {
    durationParts.microseconds = duration % 1000;
    duration = Math.floor(duration / 1000);
  }

  let momentUnits = units;
  if (units === 'ns' || units === 'mms') {
    momentUnits = 'ms';
  }
  const momentDuration = moment.duration(duration, momentUnits);

  // Get the count of each time unit that Moment handles
  allUnits
    .filterBy('inMoment')
    .mapBy('name')
    .forEach((unit) => {
      durationParts[unit] = momentDuration[unit]();
    });

  // Format each time time bucket as a string
  // e.g., { years: 5, seconds: 30 } -> [ '5 years', '30s' ]
  const displayParts = allUnits.reduce((parts, unitType) => {
    if (durationParts[unitType.name]) {
      const count = durationParts[unitType.name];
      parts.push(pluralizeUnits(count, unitType, longForm));
    }
    return parts;
  }, []);

  if (displayParts.length) {
    return displayParts.join(' ');
  }

  // When the duration is 0, show 0 in terms of `units`
  return pluralizeUnits(0, allUnits.findBy('suffix', units), longForm);
}
