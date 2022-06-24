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
          get(policy, 'rulesJSON.Namespaces'),
          this._namespace
        );
        const variables = get(policy, 'rulesJSON.Namespaces').find(
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
}
