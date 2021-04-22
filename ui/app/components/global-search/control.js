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
    const allAllocResults = results.Matches.allocs || [];
    const allGroupResults = results.Matches.groups || [];
    const allPluginResults = results.Matches.plugins || [];

    const jobResults = allJobResults.slice(0, MAXIMUM_RESULTS);
    const nodeResults = allNodeResults.slice(0, MAXIMUM_RESULTS);
    const allocResults = allAllocResults.slice(0, MAXIMUM_RESULTS);
    const groupResults = allGroupResults.slice(0, MAXIMUM_RESULTS);
    const pluginResults = allPluginResults.slice(0, MAXIMUM_RESULTS);

    jobResults.forEach(job => job.type = 'job');
    nodeResults.forEach(node => node.type = 'node');
    allocResults.forEach(alloc => alloc.type = 'alloc');
    groupResults.forEach(group => group.type = 'group');
    pluginResults.forEach(plugin => plugin.type = 'plugin');

    const truncations = results.Truncations;

    return [
      {
        groupName: resultsGroupLabel('Jobs', jobResults, allJobResults, truncations.jobs),
        options: jobResults,
      },
      {
        groupName: resultsGroupLabel('Clients', nodeResults, allNodeResults, truncations.nodes),
        options: nodeResults,
      },
      {
        groupName: resultsGroupLabel('Allocations', allocResults, allAllocResults, truncations.allocs),
        options: allocResults,
      },
      {
        groupName: resultsGroupLabel('Task Groups', groupResults, allGroupResults, truncations.groups),
        options: groupResults,
      },
      {
        groupName: resultsGroupLabel('CSI Plugins', pluginResults, allPluginResults, truncations.plugins),
        options: pluginResults,
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
      this.router.transitionTo('jobs.job', model.Scope[1], {
        queryParams: { namespace: model.Scope[0] },
      });
    } else if (model.type === 'node') {
      this.router.transitionTo('clients.client', model.Scope[0]);
    } else if (model.type === 'group') {
      this.router.transitionTo('jobs.job.task-group', model.Scope[1], model.ID, {
        queryParams: { namespace: model.Scope[0] },
      });
    } else if (model.type === 'plugin') {
      this.router.transitionTo('csi.plugins.plugin', model.ID);
    } else if (model.type === 'alloc') {
      this.router.transitionTo('allocations.allocation', model.Scope[1]);
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
