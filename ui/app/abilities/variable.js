import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import AbstractAbility from './abstract';

const WILDCARD_GLOB = '*';
const WILDCARD_PATTERN = '/';
const GLOBAL_FLAG = 'g';
const WILDCARD_PATTERN_REGEX = new RegExp(WILDCARD_PATTERN, GLOBAL_FLAG);
const PATH_PATTERN_REGEX = new RegExp(/Path(.*)/);

export default class Variable extends AbstractAbility {
  // Pass in a namespace to `can` or `cannot` calls to override
  // https://github.com/minutebase/ember-can#additional-attributes
  path = '*';

  get _path() {
    if (!this.path) return '*';
    return this.path;
  }

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableView'
  )
  canList;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableCreation'
  )
  canCreate;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportVariableView() {
    return this.rulesForNamespace.some((rules) => {
      return get(rules, 'SecureVariables');
    });
  }

  @computed('rulesForNamespace.@each.capabilities', 'path')
  get policiesSupportVariableCreation() {
    const matchingPath = this._nearestMatchingPath(this.path);
    return this.rulesForNamespace.some((rules) => {
      const keyName = `SecureVariables.Path "${matchingPath}".Capabilities`;
      const capabilities = get(rules, keyName) || [];
      return capabilities.includes('create');
    });
  }

  @computed('token.selfTokenPolicies.[]', '_namespace')
  get allPaths() {
    return (get(this, 'token.selfTokenPolicies') || [])
      .toArray()
      .reduce((paths, policy) => {
        const matchingNamespace = this._findMatchingNamespace(
          get(policy, 'rulesJSON.Namespaces') || [],
          this._namespace
        );
        const variables = (get(policy, 'rulesJSON.Namespaces') || []).find(
          (namespace) => namespace.Name === matchingNamespace
        )?.SecureVariables;
        paths = { ...paths, ...variables };
        return paths;
      }, {});
  }

  _formatMatchingPathRegEx(path, wildCardPlacement = 'end') {
    const replacer = () => '\\/';
    if (wildCardPlacement === 'end') {
      const trimmedPath = path.slice(0, path.length - 1);
      const pattern = trimmedPath.replace(WILDCARD_PATTERN_REGEX, replacer);
      return pattern;
    } else {
      const trimmedPath = path.slice(1, path.length);
      const pattern = trimmedPath.replace(WILDCARD_PATTERN_REGEX, replacer);
      return pattern;
    }
  }

  _computeAllMatchingPaths(pathNames, path) {
    const matches = [];

    for (const pathName of pathNames) {
      const pathSubString = pathName.match(PATH_PATTERN_REGEX)[1];
      const sanitizedPath = JSON.parse(pathSubString);
      const doesEndWithWildcard =
        sanitizedPath[sanitizedPath.length - 1] === WILDCARD_GLOB;
      const doesStartWithWildcard = sanitizedPath[0] === WILDCARD_GLOB;

      if (doesEndWithWildcard) {
        const formattedPath = this._formatMatchingPathRegEx(sanitizedPath);
        if (path.match(formattedPath)) matches.push(sanitizedPath);
      } else if (doesStartWithWildcard) {
        const formattedPath = this._formatMatchingPathRegEx(
          sanitizedPath,
          'start'
        );
        if (path.match(formattedPath)) matches.push(sanitizedPath);
      }
    }

    return matches;
  }

  _nearestMatchingPath(path) {
    const formattedPathKey = `Path "${path}"`;
    const pathNames = Object.keys(this.allPaths);

    if (pathNames.includes(formattedPathKey)) {
      return path;
    }

    const allMatchingPaths = this._computeAllMatchingPaths(pathNames, path);

    if (!allMatchingPaths.length) return WILDCARD_GLOB;

    return allMatchingPaths.reduce((matchingPath, currentPath) => {
      if (matchingPath === '') {
        matchingPath = currentPath;
        return matchingPath;
      }
      const count = matchingPath.match(WILDCARD_PATTERN_REGEX)?.length || 0;
      if (currentPath.match(WILDCARD_PATTERN_REGEX)?.length > count) {
        matchingPath = currentPath;
      } else if (currentPath.match(WILDCARD_PATTERN_REGEX)?.length === count) {
        // Chose suffix over prefix
        if (currentPath.endsWith(WILDCARD_GLOB)) {
          matchingPath = currentPath;
        }
      }

      return matchingPath;
    });
  }

  _doesMatchPattern(pattern, path) {
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

  _computeLengthDiff(pattern, path) {
    const countGlobsInPattern = pattern
      ?.split('')
      .filter((el) => el === WILDCARD_GLOB).length;

    return path?.length - pattern?.length + countGlobsInPattern;
  }

  _smallestDifference(matches, path) {
    const sortingCallBack = (patternA, patternB) =>
      this._computeLengthDiff(patternA, path) -
      this._computeLengthDiff(patternB, path);

    const sortedMatches = matches?.sort(sortingCallBack);
    const isTie =
      this._computeLengthDiff(sortedMatches[0], path) ===
      this._computeLengthDiff(sortedMatches[1], path);
    const doesFirstMatchHaveLeadingGlob = sortedMatches[0][0] === WILDCARD_GLOB;

    return isTie && doesFirstMatchHaveLeadingGlob
      ? sortedMatches[1]
      : sortedMatches[0];
  }
}
