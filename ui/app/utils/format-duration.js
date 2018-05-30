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

  if (units === 'ns') {
    durationParts.nanoseconds = duration % 1000;
    durationParts.microseconds = Math.floor((duration % 1000000) / 1000);
    duration = Math.floor(duration / 1000000);
  } else if (units === 'mms') {
    durationParts.microseconds = duration % 1000;
    duration = Math.floor(duration / 1000);
  }

  const momentDuration = moment.duration(duration, ['ns', 'mms'].includes(units) ? 'ms' : units);

  allUnits
    .filterBy('inMoment')
    .mapBy('name')
    .forEach(unit => {
      durationParts[unit] = momentDuration[unit]();
    });

  const displayParts = allUnits.reduce((parts, unitType) => {
    if (durationParts[unitType.name]) {
      const count = durationParts[unitType.name];
      const suffix =
        count === 1 || !unitType.pluralizable ? unitType.suffix : unitType.suffix.pluralize();
      parts.push(`${count}${unitType.pluralizable ? ' ' : ''}${suffix}`);
    }
    return parts;
  }, []);

  return displayParts.join(' ');
}
