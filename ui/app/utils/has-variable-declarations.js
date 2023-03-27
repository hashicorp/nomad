export default function hasVariableDeclarationsAndReferences(spec) {
  const VAR_DECLARATION_AND_REFERENCE_PATTERN = '\\b(?:variable|var\\.)\\b';
  const expression = new RegExp(VAR_DECLARATION_AND_REFERENCE_PATTERN);

  return expression.test(spec);
}
