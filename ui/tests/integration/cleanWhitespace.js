export default function cleanWhitespace(string) {
  return string
    .replace(/\n/g, '')
    .replace(/ +/g, ' ')
    .trim();
}
