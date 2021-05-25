export function areBasePathsMatching(pathA: string, pathB: string) {
  // use .filter(Boolean) to remove any falsy values, like ""
  const [pathABase] = pathA.split('/').filter(Boolean)
  const [pathBBase] = pathB.split('/').filter(Boolean)

  return pathABase === pathBBase
}
