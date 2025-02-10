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

  /**
   * Check if the user has read access to a specific path in a specific namespace.
   * @returns {boolean}
   */
  @computed(
    'allVariablePathRules',
    'namespace',
    'path',
    'token.selfTokenPolicies'
  )
  get policiesSupportVariableRead() {
    const matchingPath = this._nearestMatchingPath(this.path);
    if (this.namespace === WILDCARD_GLOB) {
      return this.policyNamespacesIncludeVariablesCapabilities(
        this.token.selfTokenPolicies,
        ['read'],
        matchingPath
      );
    } else {
      return this.allVariablePathRules.some((rule) => {
        const ruleMatchingPath = this._nearestMatchingPath(rule.name);
        return (
          (rule.namespace === WILDCARD_GLOB ||
            rule.namespace === this.namespace) &&
          (ruleMatchingPath === WILDCARD_GLOB ||
            ruleMatchingPath === matchingPath) &&
          rule.capabilities.includes('read')
        );
      });
    }
  }

  /**
   * Check if the user has destroy access to a specific path in a specific namespace.
   * @returns {boolean}
   */
  @computed(
    'allVariablePathRules',
    'namespace',
    'path',
    'token.selfTokenPolicies'
  )
  get policiesSupportVariableDestroy() {
    const matchingPath = this._nearestMatchingPath(this.path);
    if (this.namespace === WILDCARD_GLOB) {
      return this.policyNamespacesIncludeVariablesCapabilities(
        this.token.selfTokenPolicies,
        ['destroy'],
        matchingPath
      );
    } else {
      return this.allVariablePathRules.some((rule) => {
        const ruleMatchingPath = this._nearestMatchingPath(rule.name);
        return (
          (rule.namespace === WILDCARD_GLOB ||
            rule.namespace === this.namespace) &&
          (ruleMatchingPath === WILDCARD_GLOB ||
            ruleMatchingPath === matchingPath) &&
          rule.capabilities.includes('destroy')
        );
      });
    }
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
    const variableCapabilitiesAmongNamespaces = policies
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
    return variableCapabilitiesAmongNamespaces.some((abilityList) => {
      ['write', 'read', 'destroy'];
      return capabilities.includes(abilityList); // at least one of the capabilities is included in the list
    });
  }

  /**
   * Check if the user has write access to a specific path in a specific namespace.
   * @returns {boolean}
   */
  @computed(
    'allVariablePathRules',
    'namespace',
    'path',
    'token.selfTokenPolicies'
  )
  get policiesSupportVariableWriting() {
    const matchingPath = this._nearestMatchingPath(this.path);
    if (this.namespace === WILDCARD_GLOB) {
      // Check policyNamespacesIncludeVariablesCapabilities, which is namespace-agnostic.
      return this.policyNamespacesIncludeVariablesCapabilities(
        this.token.selfTokenPolicies,
        ['write'],
        matchingPath
      );
    } else {
      // If the namespace is not wildcarded, then we dig into rules by namespace.
      return this.allVariablePathRules.some((rule) => {
        const ruleMatchingPath = this._nearestMatchingPath(rule.name);
        return (
          (rule.namespace === WILDCARD_GLOB ||
            rule.namespace === this.namespace) &&
          (ruleMatchingPath === WILDCARD_GLOB ||
            ruleMatchingPath === matchingPath) &&
          rule.capabilities.includes('write')
        );
      });
    }
  }

  /**
   * Generate a list of all the path rules for all the policies
   * that the user has access to.
   * {
   *   namespace: string,
   *   name: string,
   *   capabilities: string[],
   * }
   * @returns {Array}
   */
  @computed('token.selfTokenPolicies.[]', 'namespace')
  get allVariablePathRules() {
    return (get(this, 'token.selfTokenPolicies') || [])
      .toArray()
      .flatMap((policy) => {
        const namespaces = get(policy, 'rulesJSON.Namespaces') || [];

        return namespaces.flatMap((namespace) => {
          const variables = namespace.Variables;
          const pathNames =
            variables?.Paths?.map((path) => ({
              namespace: namespace.Name,
              name: path.PathSpec,
              capabilities: path.Capabilities,
            })) || [];

          return pathNames;
        });
      });
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
    const pathNames = this.allVariablePathRules.map((path) => path.name);
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
