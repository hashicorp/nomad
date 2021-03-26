export const BYTES_UNITS = ['Bytes', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
export const HERTZ_UNITS = ['Hz', 'KHz', 'MHz', 'GHz', 'THz', 'PHz'];

const locale = window.navigator.locale || 'en';
const decimalFormatter = new Intl.NumberFormat(locale, {
  maximumFractionDigits: 2,
});
const roundFormatter = new Intl.NumberFormat(locale, {
  maximumFractionDigits: 0,
});

const unitReducer = (number = 0, interval, units, maxUnit) => {
  if (maxUnit && units.indexOf(maxUnit) !== -1) {
    units = units.slice(0, units.indexOf(maxUnit) + 1);
  }
  let unitIndex = 0;
  while (number >= interval && unitIndex < units.length - 1) {
    number /= interval;
    unitIndex++;
  }

  return [number, units[unitIndex]];
};

export function reduceBytes(bytes = 0, maxUnitSize) {
  return unitReducer(bytes, 1024, BYTES_UNITS, maxUnitSize);
}

export function reduceHertz(hertz, maxUnitSize) {
  return unitReducer(hertz, 1000, HERTZ_UNITS, maxUnitSize);
}

// General purpose formatters meant to reduce units as much
// as possible.
export function formatBytes(bytes) {
  const [number, unit] = reduceBytes(bytes);
  return `${decimalFormatter.format(number)} ${unit}`;
}

export function formatHertz(hertz) {
  const [number, unit] = reduceHertz(hertz);
  return `${decimalFormatter.format(number)} ${unit}`;
}

// Specialized formatters meant to reduce units to the resolution
// the scheduler and job specs operate at.
export function formatScheduledBytes(bytes) {
  const [number, unit] = reduceBytes(bytes, 'MiB');
  return `${roundFormatter.format(number)} ${unit}`;
}

export function formatScheduledHertz(hertz) {
  const [number, unit] = reduceHertz(hertz, 'MHz');
  return `${roundFormatter.format(number)} ${unit}`;
}
