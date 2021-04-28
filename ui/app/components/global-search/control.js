import Component from '@ember/component';
import { classNames } from '@ember-decorators/component';
import { task } from 'ember-concurrency';
import { action, set } from '@ember/object';
import { inject as service } from '@ember/service';
import { debounce, run } from '@ember/runloop';

const SLASH_KEY = 191;
const MAXIMUM_RESULTS = 10;

@classNames('global-search-container')
export default class GlobalSearchControl extends Component {
  @service router;
  @service token;

  searchString = null;

  constructor() {
    super(...arguments);
    this['data-test-search-parent'] = true;
  }

  keyDownHandler(e) {
    const targetElementName = e.target.nodeName.toLowerCase();

    if (targetElementName != 'input' && targetElementName != 'textarea') {
      if (e.keyCode === SLASH_KEY) {
        e.preventDefault();
        this.open();
      }
    }
  }

  didInsertElement() {
    set(this, '_keyDownHandler', this.keyDownHandler.bind(this));
    document.addEventListener('keydown', this._keyDownHandler);
  }

  willDestroyElement() {
    document.removeEventListener('keydown', this._keyDownHandler);
  }

  @task(function*(string) {
    const searchResponse = yield this.token.authorizedRequest('/v1/search/fuzzy', {
      method: 'POST',
      body: JSON.stringify({
        Text: string,
        Context: 'all',
      }),
    });

    const results = yield searchResponse.json();

    const allJobResults = results.Matches.jobs || [];
    const allNodeResults = results.Matches.nodes || [];
    const allAllocationResults = results.Matches.allocs || [];
    const allTaskGroupResults = results.Matches.groups || [];
    const allCSIPluginResults = results.Matches.plugins || [];

    const jobResults = allJobResults.slice(0, MAXIMUM_RESULTS).map(({ ID: name, Scope: [ namespace, id ]}) => ({
      type: 'job',
      id,
      namespace,
      label: name,
    }));

    const nodeResults = allNodeResults.slice(0, MAXIMUM_RESULTS).map(({ ID: name, Scope: [ id ]}) => ({
      type: 'node',
      id,
      label: name,
    }));

    const allocationResults = allAllocationResults.slice(0, MAXIMUM_RESULTS).map(({ ID: name, Scope: [ , id ]}) => ({
      type: 'allocation',
      id,
      label: name,
    }));

    const taskGroupResults = allTaskGroupResults.slice(0, MAXIMUM_RESULTS).map(({ ID: id, Scope: [ namespace, jobId ]}) => ({
      type: 'task-group',
      id,
      namespace,
      jobId,
      label: id,
    }));

    const csiPluginResults = allCSIPluginResults.slice(0, MAXIMUM_RESULTS).map(({ ID: id }) => ({
      type: 'plugin',
      id,
      label: id,
    }));

    const {
      jobs: jobsTruncated,
      nodes: nodesTruncated,
      allocs: allocationsTruncated,
      groups: taskGroupsTruncated,
      plugins: csiPluginsTruncated,
    } = results.Truncations;

    return [
      {
        groupName: resultsGroupLabel('Jobs', jobResults, allJobResults, jobsTruncated),
        options: jobResults,
      },
      {
        groupName: resultsGroupLabel('Clients', nodeResults, allNodeResults, nodesTruncated),
        options: nodeResults,
      },
      {
        groupName: resultsGroupLabel('Allocations', allocationResults, allAllocationResults, allocationsTruncated),
        options: allocationResults,
      },
      {
        groupName: resultsGroupLabel('Task Groups', taskGroupResults, allTaskGroupResults, taskGroupsTruncated),
        options: taskGroupResults,
      },
      {
        groupName: resultsGroupLabel('CSI Plugins', csiPluginResults, allCSIPluginResults, csiPluginsTruncated),
        options: csiPluginResults,
      }
    ];
  })
  search;

  @action
  open() {
    if (this.select) {
      this.select.actions.open();
    }
  }

  @action
  ensureMinimumLength(string) {
    return string.length > 1;
  }

  @action
  selectOption(model) {
    if (model.type === 'job') {
      this.router.transitionTo('jobs.job', model.id, {
        queryParams: { namespace: model.namespace },
      });
    } else if (model.type === 'node') {
      this.router.transitionTo('clients.client', model.id);
    } else if (model.type === 'task-group') {
      this.router.transitionTo('jobs.job.task-group', model.jobId, model.id, {
        queryParams: { namespace: model.namespace },
      });
    } else if (model.type === 'plugin') {
      this.router.transitionTo('csi.plugins.plugin', model.id);
    } else if (model.type === 'allocation') {
      this.router.transitionTo('allocations.allocation', model.id);
    }
  }

  @action
  storeSelect(select) {
    if (select) {
      this.select = select;
    }
  }

  @action
  openOnClickOrTab(select, { target }) {
    // Bypass having to press enter to access search after clicking/tabbing
    const targetClassList = target.classList;
    const targetIsTrigger = targetClassList.contains('ember-power-select-trigger');

    // Allow tabbing out of search
    const triggerIsNotActive = !targetClassList.contains('ember-power-select-trigger--active');

    if (targetIsTrigger && triggerIsNotActive) {
      debounce(this, this.open, 150);
    }
  }

  @action
  onCloseEvent(select, event) {
    if (event.key === 'Escape') {
      run.next(() => {
        this.element.querySelector('.ember-power-select-trigger').blur();
      });
    }
  }

  calculatePosition(trigger) {
    const { top, left, width } = trigger.getBoundingClientRect();
    return {
      style: {
        left,
        width,
        top,
      },
    };
  }
}

function resultsGroupLabel(type, renderedResults, allResults, truncated) {
  let countString;

  if (renderedResults.length < allResults.length) {
    countString = `showing ${renderedResults.length} of ${allResults.length}`;
  } else {
    countString = renderedResults.length;
  }

  const truncationIndicator = truncated ? '+' : '';

  return `${type} (${countString}${truncationIndicator})`;
}
