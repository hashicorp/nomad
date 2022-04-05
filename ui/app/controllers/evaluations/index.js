import { getOwner } from '@ember/application';
import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';
import { inject as service } from '@ember/service';
import { useMachine } from 'ember-statecharts';
import { use } from 'ember-usable';
import evaluationsMachine from '../../machines/evaluations';

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

  queryParams = ['nextToken', 'currentEval', 'pageSize', 'status'];
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
      (e instanceof KeyboardEvent && (e.code === 'Enter' || e.code === 'Space'))
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

  @tracked pageSize = this.userSettings.pageSize;
  @tracked nextToken = null;
  @tracked previousTokens = [];
  @tracked status = null;

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
  setStatus(selection) {
    this._resetTokens();
    this.status = selection;
  }

  _resetTokens() {
    this.nextToken = null;
    this.previousTokens = [];
  }
}
