/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object/computed';
import { action, computed } from '@ember/object';
import classic from 'ember-classic-decorator';
import { tracked } from '@glimmer/tracking';

import { serialize } from 'nomad-ui/utils/qp-serialize';

const alertClassFallback = 'is-info';

const errorLevelToAlertClass = {
  danger: 'is-danger',
  warn: 'is-warning',
};

@classic
export default class VersionsController extends Controller.extend(
  WithNamespaceResetting
) {
  error = null;

  @alias('model') job;

  queryParams = ['diffVersion'];

  @computed('error.level')
  get errorLevelClass() {
    return (
      errorLevelToAlertClass[this.get('error.level')] || alertClassFallback
    );
  }

  onDismiss() {
    this.set('error', null);
  }

  @action
  handleError(errorObject) {
    this.set('error', errorObject);
  }

  @tracked diffVersion = '';

  get optionsDiff() {
    return this.job.versions.map((version) => {
      return {
        label: version.versionTag?.name || `version ${version.number}`,
        value: String(version.number),
      };
    });
  }

  @tracked diffs = [];

  @action async versionsDidUpdate() {
    try {
      const diffs = await this.job.getVersions(this.diffVersion);
      if (diffs.Diffs) {
        this.diffs = diffs.Diffs;
      } else {
        this.diffs = [];
      }
    } catch (error) {
      console.error('error fetching diffs', error);
    }
  }

  get diffsExpanded() {
    return this.diffVersion !== '';
  }

  @action setDiffVersion(label) {
    if (!label) {
      this.diffVersion = '';
    } else {
      this.diffVersion = serialize(label);
    }
  }
}
