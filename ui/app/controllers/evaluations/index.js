import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { matchesState, useMachine } from 'ember-statecharts';
import { use } from 'ember-usable';
import evaluationsMachine from '../../machines/evaluations';

export default class EvaluationsController extends Controller {
  @service store;
  @service userSettings;

  @matchesState({ sidebar: 'open' })
  isSideBarOpen;

  @use statechart = useMachine(evaluationsMachine).withConfig({
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
    });
  }

  @action
  closeSidebar() {
    return this.statechart.send('MODAL_CLOSE');
  }

  @action
  async handleEvaluationClick(evaluation) {
    this.statechart.send('LOAD_EVALUATION', { evaluation });
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

  get currentEvalDetail() {
    return {
      ID: this.currentEval,
      Priority: 50,
      Type: 'service',
      TriggeredBy: 'job-register',
      JobID: 'example',
      JobModifyIndex: 52,
      NodeID: '',
      NodeModifyIndex: 0,
      Status: 'complete',
      StatusDescription: '',
      Wait: 0,
      NextEval: '',
      PreviousEval: '',
      BlockedEval: '',
      FailedTGAllocs: null,
      ClassEligibility: null,
      EscapedComputedClass: false,
      AnnotatePlan: false,
      SnapshotIndex: 53,
      QueuedAllocations: {
        cache: 0,
      },
      CreateIndex: 53,
      ModifyIndex: 55,
      RelatedEvals: [
        {
          CreateIndex: 82,
          CreateTime: 1646071501471847000,
          ID: 'eef1147c-3396-928d-b0f3-ce411bd5d550',
          JobID: 'system-job',
          ModifyIndex: 88,
          ModifyTime: 1646071501474137000,
          Namespace: 'default',
          NodeID: '462aad3f-bfa5-00be-d1d4-aa713adb4dbe',
          NodeModifyIndex: 81,
          Priority: 50,
          QueuedAllocations: {
            cache: 0,
          },
          SnapshotIndex: 82,
          Status: 'complete',
          TriggeredBy: 'node-update',
          Type: 'system',
        },
        {
          CreateIndex: 82,
          CreateTime: 1646071501471847000,
          DeploymentID: '61adb5d0-4bb2-fe37-f504-21e23377e291',
          ID: '55d51a54-f7af-f978-2390-c5688a17fba8',
          JobID: 'example2',
          ModifyIndex: 90,
          ModifyTime: 1646071501476093000,
          Namespace: 'default',
          NodeID: '462aad3f-bfa5-00be-d1d4-aa713adb4dbe',
          NodeModifyIndex: 81,
          Priority: 50,
          QueuedAllocations: {
            cache: 0,
          },
          SnapshotIndex: 89,
          Status: 'complete',
          TriggeredBy: 'node-update',
          Type: 'service',
        },
        {
          CreateIndex: 82,
          CreateTime: 1646071501471847000,
          DeploymentID: 'c60e9405-86ff-5de6-9d3e-ab4ab2409d28',
          ID: '2b33210a-9471-b523-0ae4-e8fcc9d094fe',
          JobID: 'example',
          ModifyIndex: 86,
          ModifyTime: 1646071501473893000,
          Namespace: 'default',
          NodeID: '462aad3f-bfa5-00be-d1d4-aa713adb4dbe',
          NodeModifyIndex: 81,
          Priority: 50,
          QueuedAllocations: {
            cache: 0,
          },
          SnapshotIndex: 84,
          Status: 'complete',
          TriggeredBy: 'node-update',
          Type: 'service',
        },
        {
          CreateIndex: 43,
          CreateTime: 1646071122930603000,
          DeploymentID: '61adb5d0-4bb2-fe37-f504-21e23377e291',
          ID: 'f8915b6d-294b-a60d-172d-f0b5d4e6df1a',
          JobID: 'example2',
          ModifyIndex: 46,
          ModifyTime: 1646071122936311000,
          Namespace: 'default',
          NodeID: '462aad3f-bfa5-00be-d1d4-aa713adb4dbe',
          NodeModifyIndex: 42,
          Priority: 50,
          QueuedAllocations: {
            cache: 0,
          },
          SnapshotIndex: 45,
          Status: 'complete',
          TriggeredBy: 'node-update',
          Type: 'service',
        },
      ],
    };
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
