/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { getOwner } from '@ember/application';
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';
import { inject as service } from '@ember/service';
import { useMachine } from 'ember-statecharts';
import { use } from 'ember-usable';
import evaluationsMachine from '../../machines/evaluations';

const ALL_NAMESPACE_WILDCARD = '*';

export default class EvaluationsController extends Controller {
  @service store;
  @service userSettings;

  // We use statecharts here to manage complex user flows for the sidebar logic
  @use
  statechart = useMachine(evaluationsMachine).withConfig({
    services: {
      loadEvaluation: this.loadEvaluation,
    },
    actions: {
      updateEvaluationQueryParameter: this.updateEvaluationQueryParameter,
      removeCurrentEvaluationQueryParameter:
        this.removeCurrentEvaluationQueryParameter,
    },
    guards: {
      sidebarIsOpen: this._sidebarIsOpen,
    },
  });

  queryParams = [
    'nextToken',
    'currentEval',
    'pageSize',
    'status',
    { qpNamespace: 'namespace' },
    'type',
    'searchTerm',
  ];
  @tracked currentEval = null;

  @action
  _sidebarIsOpen() {
    return !!this.currentEval;
  }

  @action
  async loadEvaluation(context, { evaluation }) {
    let evaluationId;
    if (evaluation?.id) {
      evaluationId = evaluation.id;
    } else {
      evaluationId = this.currentEval;
    }

    return this.store.findRecord('evaluation', evaluationId, {
      reload: true,
      adapterOptions: { related: true },
    });
  }

  @action
  async handleEvaluationClick(evaluation, e) {
    if (
      e instanceof MouseEvent ||
      (e instanceof KeyboardEvent &&
        (e.code === 'Enter' || e.code === 'Space')) ||
      !e
    ) {
      this.statechart.send('LOAD_EVALUATION', { evaluation });
    }
  }

  @action
  notifyEvalChange([evaluation]) {
    schedule('actions', this, () => {
      this.statechart.send('CHANGE_EVAL', { evaluation });
    });
  }

  @action
  updateEvaluationQueryParameter(context, { evaluation }) {
    this.currentEval = evaluation.id;
  }

  @action
  removeCurrentEvaluationQueryParameter() {
    this.currentEval = null;
  }

  get shouldDisableNext() {
    return !this.model.meta?.nextToken;
  }

  get shouldDisablePrev() {
    return !this.previousTokens.length;
  }

  get optionsEvaluationsStatus() {
    return [
      { key: null, label: 'All' },
      { key: 'blocked', label: 'Blocked' },
      { key: 'pending', label: 'Pending' },
      { key: 'complete', label: 'Complete' },
      { key: 'failed', label: 'Failed' },
      { key: 'canceled', label: 'Canceled' },
    ];
  }

  get optionsTriggeredBy() {
    return [
      { key: null, label: 'All' },
      { key: 'job-register', label: 'Job Register' },
      { key: 'job-deregister', label: 'Job Deregister' },
      { key: 'periodic-job', label: 'Periodic Job' },
      { key: 'node-drain', label: 'Node Drain' },
      { key: 'node-update', label: 'Node Update' },
      { key: 'alloc-stop', label: 'Allocation Stop' },
      { key: 'scheduled', label: 'Scheduled' },
      { key: 'rolling-update', label: 'Rolling Update' },
      { key: 'deployment-watcher', label: 'Deployment Watcher' },
      { key: 'failed-follow-up', label: 'Failed Follow Up' },
      { key: 'max-disconnect-timeout', label: 'Max Disconnect Timeout' },
      { key: 'max-plan-attempts', label: 'Max Plan Attempts' },
      { key: 'alloc-failure', label: 'Allocation Failure' },
      { key: 'queued-allocs', label: 'Queued Allocations' },
      { key: 'preemption', label: 'Preemption' },
      { key: 'job-scaling', label: 'Job Scalling' },
    ];
  }

  get optionsNamespaces() {
    const namespaces = this.store.peekAll('namespace').map((namespace) => ({
      key: namespace.name,
      label: namespace.name,
    }));

    // Create default namespace selection
    namespaces.unshift({
      key: ALL_NAMESPACE_WILDCARD,
      label: 'All (*)',
    });

    return namespaces;
  }

  get optionsType() {
    return [
      { key: null, label: 'All' },
      { key: 'client', label: 'Client' },
      { key: 'no client', label: 'No Client' },
    ];
  }

  filters = ['status', 'qpNamespace', 'type', 'triggeredBy', 'searchTerm'];

  get hasFiltersApplied() {
    return this.filters.reduce((result, filter) => {
      // By default we always set qpNamespace to the '*' wildcard
      // We need to ensure that if namespace is the only filter, that we send the correct error message to the user
      if (this[filter] && filter !== 'qpNamespace') {
        result = true;
      }
      return result;
    }, false);
  }

  get currentFilters() {
    const result = [];
    for (const filter of this.filters) {
      const isNamespaceWildcard =
        filter === 'qpNamespace' && this[filter] === '*';
      if (this[filter] && !isNamespaceWildcard) {
        result.push({ [filter]: this[filter] });
      }
    }
    return result;
  }

  get noMatchText() {
    let text = '';
    const cleanNames = {
      status: 'Status',
      qpNamespace: 'Namespace',
      type: 'Type',
      triggeredBy: 'Triggered By',
      searchTerm: 'Search Term',
    };
    if (this.hasFiltersApplied) {
      for (let i = 0; i < this.currentFilters.length; i++) {
        const filter = this.currentFilters[i];
        const [name] = Object.keys(filter);
        const filterName = cleanNames[name];
        const filterValue = filter[name];
        if (this.currentFilters.length === 1)
          return `${filterName}: ${filterValue}.`;
        if (i !== 0 && i !== this.currentFilters.length - 1)
          text = text.concat(`, ${filterName}: ${filterValue}`);
        if (i === 0) text = text.concat(`${filterName}: ${filterValue}`);
        if (i === this.currentFilters.length - 1) {
          return text.concat(`, ${filterName}: ${filterValue}.`);
        }
      }
    }

    return text;
  }

  @tracked pageSize = this.userSettings.pageSize;
  @tracked nextToken = null;
  @tracked previousTokens = [];
  @tracked status = null;
  @tracked triggeredBy = null;
  @tracked qpNamespace = ALL_NAMESPACE_WILDCARD;
  @tracked type = null;
  @tracked searchTerm = null;

  @action
  onChange(newPageSize) {
    this.pageSize = newPageSize;
  }

  @action
  onNext(nextToken) {
    this.previousTokens = [...this.previousTokens, this.nextToken];
    this.nextToken = nextToken;
  }

  @action
  onPrev() {
    const lastToken = this.previousTokens.pop();
    this.previousTokens = [...this.previousTokens];
    this.nextToken = lastToken;
  }

  @action
  refresh() {
    const isDefaultParams = this.nextToken === null && this.status === null;
    if (isDefaultParams) {
      getOwner(this).lookup('route:evaluations.index').refresh();
      return;
    }

    this._resetTokens();
    this.status = null;
    this.pageSize = this.userSettings.pageSize;
  }

  @action
  setQueryParam(qp, selection) {
    this._resetTokens();
    this[qp] = selection;
  }

  @action
  toggle() {
    this._resetTokens();
    this.shouldOnlyDisplayClientEvals = !this.shouldOnlyDisplayClientEvals;
  }

  @action
  _resetTokens() {
    this.nextToken = null;
    this.previousTokens = [];
  }
}
