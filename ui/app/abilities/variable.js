/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { computed, get } from '@ember/object';
import { or } from '@ember/object/computed';
import AbstractAbility from './abstract';
import doesMatchPattern from 'nomad-ui/utils/match-glob';

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
    'policiesSupportVariableList'
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

  @or(
    'bypassAuthorization',
    'selfTokenIsManagement',
    'policiesSupportVariableRead'
  )
  canRead;

  @computed('token.selfTokenPolicies')
  get policiesSupportVariableList() {
    return this.policyNamespacesIncludeVariablesCapabilities(
      this.token.selfTokenPolicies,
      ['list', 'read', 'write', 'destroy']
    );
  }

  @computed('path', 'allPaths')
  get policiesSupportVariableRead() {
    const matchingPath = this._nearestMatchingPath(this.path);
    return this.allPaths
      .find((path) => path.name === matchingPath)
      ?.capabilities?.includes('read');
  }

  /**
   *
   * Map to your policy's namespaces,
   * and each of their Variables blocks' paths,
   * and each of their capabilities.
   * Then, check to see if any of the permissions you're looking for
   * are contained within at least one of them.
   *
   * @param {Object} policies
   * @param {string[]} capabilities
   * @returns {boolean}
   */
  policyNamespacesIncludeVariablesCapabilities(
    policies = [],
    capabilities = [],
    path
  ) {
    const namespacesWithVariableCapabilities = policies
      .toArray()
      .filter((policy) => get(policy, 'rulesJSON.Namespaces'))
      .map((policy) => get(policy, 'rulesJSON.Namespaces'))
      .flat()
      .map((namespace = {}) => {
        return namespace.Variables?.Paths;
      })
      .flat()
      .compact()
      .filter((varsBlock = {}) => {
        if (!path || path === WILDCARD_GLOB) {
          return true;
        } else {
          return varsBlock.PathSpec === path;
        }
      })
      .map((varsBlock = {}) => {
        return varsBlock.Capabilities;
      })
      .flat()
      .compact();

    // Check for requested permissions
    return namespacesWithVariableCapabilities.some((abilityList) => {
      return capabilities.includes(abilityList);
    });
  }

  @computed('allPaths', 'namespace', 'path', 'token.selfTokenPolicies')
  get policiesSupportVariableWriting() {
    if (this.namespace === WILDCARD_GLOB && this.path === WILDCARD_GLOB) {
      // If you're checking if you can write from root, and you don't specify a namespace,
      // Then if you can write in ANY path in ANY namespace, you can get to /new.
      return this.policyNamespacesIncludeVariablesCapabilities(
        this.token.selfTokenPolicies,
        ['write'],
        this._nearestMatchingPath(this.path)
      );
    } else {
      // Checking a specific path in a specific namespace.
      // TODO: This doesn't cover the case when you're checking for the * namespace at a specific path.
      // Right now we require you to specify yournamespace to enable the button.
      const matchingPath = this._nearestMatchingPath(this.path);
      return this.allPaths
        .find((path) => path.name === matchingPath)
        ?.capabilities?.includes('write');
    }
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
        const namespaces = get(policy, 'rulesJSON.Namespaces');
        const matchingNamespace = this._nearestMatchingNamespace(
          namespaces,
          this.namespace
        );

        const variables = (namespaces || []).find(
          (namespace) => namespace.Name === matchingNamespace
        )?.Variables;

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

  _nearestMatchingNamespace(policyNamespaces, namespace) {
    if (!namespace || !policyNamespaces) return 'default';

    return this._findMatchingNamespace(policyNamespaces, namespace);
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
      if (doesMatchPattern(pathName, path)) matches.push(pathName);
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
