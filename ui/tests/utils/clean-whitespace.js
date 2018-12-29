// cleans whitespace from a string, for example for cleaning
// textContent in DOM nodes with indentation
export default function cleanWhitespace(string) {
  return string
    .replace(/\n/g, '')
    .replace(/ +/g, ' ')
    .trim();
}
