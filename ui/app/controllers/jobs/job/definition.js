/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import classic from 'ember-classic-decorator';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

/**
 * Controller for handling job definition and specification, along with editing state and view.
 * @augments Controller
 */
@classic
export default class DefinitionController extends Controller.extend(
  WithNamespaceResetting
) {
  @alias('model.definition') definition;
  @alias('model.format') format;
  @alias('model.job') job;
  @alias('model.specification') specification;
  @alias('model.variableFlags') variableFlags;
  @alias('model.variableLiteral') variableLiteral;

  @tracked view;
  @tracked isEditing = false;
  queryParams = ['isEditing', 'view'];

  @service router;

  /**
   * Get the context of the controller based on the editing state.
   * @returns {"edit"|"read"} The context, either 'edit' or 'read'.
   */
  get context() {
    return this.isEditing ? 'edit' : 'read';
  }

  /**
   * Toggle the editing state.
   * @param {boolean} [bool] - Optional boolean value to set the editing state.
   */
  @action
  toggleEdit(bool) {
    this.isEditing = bool || !this.isEditing;
  }

  /**
   * Update the view based on the selected view.
   * @param {"job-spec" | "full-definition"} selectedView - The selected view, either 'job-spec' or 'full-definition'.
   */
  @action
  selectView(selectedView) {
    this.view = selectedView;
  }

  onSubmit() {
    this.router.transitionTo('jobs.job', this.job.idWithNamespace);
  }
}
