// @ts-check

const WILDCARD_GLOB = '*';

export default function matchGlob(pattern, path) {
  console.log('matchGlob called with', pattern, path);
  const parts = pattern?.split(WILDCARD_GLOB);
  const hasLeadingGlob = pattern?.startsWith(WILDCARD_GLOB);
  const hasTrailingGlob = pattern?.endsWith(WILDCARD_GLOB);
  const lastPartOfPattern = parts[parts.length - 1];
  const isPatternWithoutGlob = parts.length === 1 && !hasLeadingGlob;

  if (!pattern || !path || isPatternWithoutGlob) {
    return pattern === path;
  }

  if (pattern === WILDCARD_GLOB) {
    return true;
  }

  let subPathToMatchOn = path;
  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    const idx = subPathToMatchOn?.indexOf(part);
    const doesPathIncludeSubPattern = idx > -1;
    const doesMatchOnFirstSubpattern = idx === 0;

    if (i === 0 && !hasLeadingGlob && !doesMatchOnFirstSubpattern) {
      return false;
    }

    if (!doesPathIncludeSubPattern) {
      return false;
    }

    subPathToMatchOn = subPathToMatchOn.slice(0, idx + path.length);
  }

  return hasTrailingGlob || path.endsWith(lastPartOfPattern);
}

// _doesMatchPattern(pattern, path) {
//   const parts = pattern?.split(WILDCARD_GLOB);
//   const hasLeadingGlob = pattern?.startsWith(WILDCARD_GLOB);
//   const hasTrailingGlob = pattern?.endsWith(WILDCARD_GLOB);
//   const lastPartOfPattern = parts[parts.length - 1];
//   const isPatternWithoutGlob = parts.length === 1 && !hasLeadingGlob;

//   if (!pattern || !path || isPatternWithoutGlob) {
//     return pattern === path;
//   }

//   if (pattern === WILDCARD_GLOB) {
//     return true;
//   }

//   let subPathToMatchOn = path;
//   for (let i = 0; i < parts.length; i++) {
//     const part = parts[i];
//     const idx = subPathToMatchOn?.indexOf(part);
//     const doesPathIncludeSubPattern = idx > -1;
//     const doesMatchOnFirstSubpattern = idx === 0;

//     if (i === 0 && !hasLeadingGlob && !doesMatchOnFirstSubpattern) {
//       return false;
//     }

//     if (!doesPathIncludeSubPattern) {
//       return false;
//     }

//     subPathToMatchOn = subPathToMatchOn.slice(0, idx + path.length);
//   }

//   return hasTrailingGlob || path.endsWith(lastPartOfPattern);
// }
