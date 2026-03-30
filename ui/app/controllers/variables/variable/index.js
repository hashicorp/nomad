/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { set, action } from '@ember/object';
import { task } from 'ember-concurrency';
import { service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

export default class VariablesVariableIndexController extends Controller {
  @service router;

  queryParams = ['view', 'sortProperty', 'sortDescending'];

  @tracked sortProperty = 'key';
  @tracked sortDescending = true;

  @service notifications;

  get sortedKeyValues() {
    const sorted = [...this.model.keyValues].sort((a, b) => {
      const aVal = a[this.sortProperty];
      const bVal = b[this.sortProperty];

      let comparison = 0;
      if (typeof aVal === 'string' && typeof bVal === 'string') {
        comparison = aVal?.localeCompare(bVal) || 0;
      } else {
        comparison = (aVal || 0) - (bVal || 0);
      }

      return this.sortDescending ? comparison : -comparison;
    });
    return sorted;
  }

  @tracked isDeleting = false;

  @action
  onDeletePrompt() {
    this.isDeleting = true;
  }

  @action
  onDeleteCancel() {
    this.isDeleting = false;
  }

  @action copyVariable() {
    navigator.clipboard.writeText(JSON.stringify(this.model.items, null, 2));
  }

  @task(function* () {
    try {
      yield this.model.deleteRecord();
      yield this.model.save();
      if (this.model.parentFolderPath) {
        this.router.transitionTo('variables.path', this.model.parentFolderPath);
      } else {
        this.router.transitionTo('variables');
      }
      this.notifications.add({
        title: 'Variable deleted',
        message: `${this.model.path} successfully deleted`,
        color: 'success',
      });
    } catch (err) {
      this.notifications.add({
        title: `Error deleting ${this.model.path}`,
        message: err,
        color: 'critical',
        sticky: true,
      });
    }
  })
  deleteVariableFile;

  //#region Code View
  /**
   * @type {"table" | "json"}
   */
  @tracked
  view = 'table';

  @action
  toggleView() {
    if (this.view === 'table') {
      this.view = 'json';
    } else {
      this.view = 'table';
    }
  }
  //#endregion Code View

  get shouldShowLinkedEntities() {
    return (
      this.model.pathLinkedEntities?.job ||
      this.model.pathLinkedEntities?.group ||
      this.model.pathLinkedEntities?.task ||
      this.model.path === 'nomad/jobs'
    );
  }

  @action
  toggleRowVisibility(kv) {
    set(kv, 'isVisible', !kv.isVisible);
  }
}
