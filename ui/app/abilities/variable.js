import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import AbstractAbility from './abstract';

const WILDCARD_GLOB = '*';
const WILDCARD_PATTERN = '/';
const GLOBAL_FLAG = 'g';
const WILDCARD_PATTERN_REGEX = new RegExp(WILDCARD_PATTERN, GLOBAL_FLAG);

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
    'policiesSupportVariableWriting'
  )
  canWrite;

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableDestroy'
  )
  canDestroy;

  @computed('rulesForNamespace.@each.capabilities')
  get policiesSupportVariableView() {
    return this.rulesForNamespace.some((rules) => {
      return get(rules, 'SecureVariables');
    });
  }

  @computed('path', 'allPaths')
  get policiesSupportVariableWriting() {
    const matchingPath = this._nearestMatchingPath(this.path);
    return this.allPaths
      .find((path) => path.name === matchingPath)
      ?.capabilities?.includes('write');
  }

  @computed('path', 'allPaths')
  get policiesSupportVariableDestroy() {
    const matchingPath = this._nearestMatchingPath(this.path);
    return this.allPaths
      .find((path) => path.name === matchingPath)
      ?.capabilities?.includes('destroy');
  }

  @computed('token.selfTokenPolicies.[]', 'namespace')
  get allPaths() {
    return (get(this, 'token.selfTokenPolicies') || [])
      .toArray()
      .reduce((paths, policy) => {
        const matchingNamespace = this.namespace ?? 'default';

        const variables = (get(policy, 'rulesJSON.Namespaces') || []).find(
          (namespace) => namespace.Name === matchingNamespace
        )?.SecureVariables;

        const pathNames = variables?.Paths?.map((path) => ({
          name: path.PathSpec,
          capabilities: path.Capabilities,
        }));

        if (pathNames) {
          paths = [...paths, ...pathNames];
        }

        return paths;
      }, []);
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
      if (this._doesMatchPattern(pathName, path)) matches.push(pathName);
    }

    return matches;
  }

  _nearestMatchingPath(path) {
    const pathNames = this.allPaths.map((path) => path.name);

    if (pathNames.includes(path)) {
      return path;
    }

    const allMatchingPaths = this._computeAllMatchingPaths(pathNames, path);

    if (!allMatchingPaths.length) return WILDCARD_GLOB;

    return this._smallestDifference(allMatchingPaths, path);
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
