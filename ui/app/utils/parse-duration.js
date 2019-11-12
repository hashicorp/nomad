const unitToMs = {
  ms: 1,
  s: 1000,
  m: 1000 * 60,
  h: 1000 * 60 * 60,
  d: 1000 * 60 * 60 * 24,
};
const durationUnits = Object.keys(unitToMs);

const isNumeric = char => char >= 0 && char < 10;

const encodeUnit = (str, token) => {
  // Convert it to a string and validate the unit type.
  let newToken = token.join('');
  if (!durationUnits.includes(newToken)) {
    throw new Error(`ParseError: [${str}] Unallowed duration unit "${newToken}"`);
  }
  return newToken;
};

const encodeQuantity = (str, token) => {
  return parseInt(token.join(''));
};

export default str => {
  if (typeof str === 'number') return str;

  // Split the string into characters to make iteration easier
  const chars = str.split('');

  // Bail early if the duration doesn't start with a number
  if (!isNumeric(chars[0])) {
    throw new Error(`ParseError: [${str}] Durations must start with a numeric quantity`);
  }

  // Collect tokens
  const tokens = [];

  // A token can be multi-character, so collect characters
  let token = [];

  // Alternate between numeric "quantity" tokens and non-numeric "unit" tokens
  let unitMode = false;

  while (chars.length) {
    let finishToken = false;
    let next = chars.shift();

    // First identify if the next character is the first
    // character of the next token.
    if (isNumeric(next) && unitMode) {
      unitMode = false;
      finishToken = true;
    } else if (!isNumeric(next) && !unitMode) {
      unitMode = true;
      finishToken = true;
    }

    // When a token is finished, validate it, encode it, and add it to the tokens list
    if (finishToken) {
      tokens.push(unitMode ? encodeQuantity(str, token) : encodeUnit(str, token));
      token = [];
    }

    // Always add the next character to the token buffer.
    token.push(next);
  }

  // Once the loop finishes, flush the token buffer one more time.
  if (unitMode) {
    tokens.push(encodeUnit(str, token));
  } else {
    throw new Error(`ParseError: [${str}] Unmatched quantities and units`);
  }

  // Loop over the tokens array, two at a time, converting unit and quanties into milliseconds
  let duration = 0;
  while (tokens.length) {
    const quantity = tokens.shift();
    const unit = tokens.shift();
    duration += quantity * unitToMs[unit];
  }

  // Convert from Milliseconds to Nanoseconds
  duration *= 1000000;

  return duration;
};
