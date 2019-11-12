export default str => {
  const durationUnits = ['d', 'h', 'm', 's'];
  const unitToMs = {
    s: 1000,
    m: 1000 * 60,
    h: 1000 * 60 * 60,
    d: 1000 * 60 * 60 * 24,
  };

  if (typeof str === 'number') return str;

  // Split the string into characters to make iteration easier
  const chars = str.split('');

  // Collect tokens
  const tokens = [];

  // A token can be multi-character, so collect characters
  let token = [];

  // If a non-numeric character follows a non-numeric character, that's a
  // parse error, so this marker bool is needed
  let disallowChar = false;

  // Take the first character off the chars array until there are no more
  while (chars.length) {
    let next = chars.shift();

    // Check to see if the char is numeric
    if (next >= 0 && next < 10) {
      // Collect numeric characters until a non-numeric shows up
      token.push(next);
      // Release the double non-numeric mark
      disallowChar = false;
    } else {
      if (disallowChar) {
        throw new Error(
          `ParseError: [${str}] Cannot follow a non-numeric token with a non-numeric token`
        );
      }
      if (!durationUnits.includes(next)) {
        throw new Error(`ParseError: [${str}] Unallowed duration unit "${next}"`);
      }

      // The token array now becomes a single int token
      tokens.push(parseInt(token.join('')));
      // This non-numeric char is its own token
      tokens.push(next);

      // Reset token array
      token = [];
      // Set the double non-numeric mark
      disallowChar = true;
    }
  }

  // If there are numeric characters still in the token array, then there must have
  // not been a followup non-numeric character which would have flushed the numeric tokens.
  if (token.length) {
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
