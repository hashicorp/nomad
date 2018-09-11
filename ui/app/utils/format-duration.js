import moment from 'moment';

const allUnits = [
  { name: 'years', suffix: 'year', inMoment: true, pluralizable: true },
  { name: 'months', suffix: 'month', inMoment: true, pluralizable: true },
  { name: 'days', suffix: 'day', inMoment: true, pluralizable: true },
  { name: 'hours', suffix: 'h', inMoment: true, pluralizable: false },
  { name: 'minutes', suffix: 'm', inMoment: true, pluralizable: false },
  { name: 'seconds', suffix: 's', inMoment: true, pluralizable: false },
  { name: 'milliseconds', suffix: 'ms', inMoment: true, pluralizable: false },
  { name: 'microseconds', suffix: 'µs', inMoment: false, pluralizable: false },
  { name: 'nanoseconds', suffix: 'ns', inMoment: false, pluralizable: false },
];

export default function formatDuration(duration = 0, units = 'ns') {
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
    .forEach(unit => {
      durationParts[unit] = momentDuration[unit]();
    });

  // Format each time time bucket as a string
  // e.g., { years: 5, seconds: 30 } -> [ '5 years', '30s' ]
  const displayParts = allUnits.reduce((parts, unitType) => {
    if (durationParts[unitType.name]) {
      const count = durationParts[unitType.name];
      const suffix =
        count === 1 || !unitType.pluralizable ? unitType.suffix : unitType.suffix.pluralize();
      parts.push(`${count}${unitType.pluralizable ? ' ' : ''}${suffix}`);
    }
    return parts;
  }, []);

  if (displayParts.length) {
    return displayParts.join(' ');
  }

  // When the duration is 0, show 0 in terms of `units`
  const unitTypeForUnits = allUnits.findBy('suffix', units);
  const suffix = unitTypeForUnits.pluralizable ? units.pluralize() : units;
  return `0${unitTypeForUnits.pluralizable ? ' ' : ''}${suffix}`;
}
