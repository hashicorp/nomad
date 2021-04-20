import Component from '@ember/component';
import { classNames } from '@ember-decorators/component';
import { task } from 'ember-concurrency';
import EmberObject, { action, computed, set } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { debounce, run } from '@ember/runloop';
import Searchable from 'nomad-ui/mixins/searchable';
import classic from 'ember-classic-decorator';

const SLASH_KEY = 191;
const MAXIMUM_RESULTS = 10;

@classNames('global-search-container')
export default class GlobalSearchControl extends Component {
  @service dataCaches;
  @service router;
  @service store;
  @service token;

  searchString = null;

  constructor() {
    super(...arguments);
    this['data-test-search-parent'] = true;

    this.jobSearch = JobSearch.create({
      dataSource: this,
    });

    this.nodeNameSearch = NodeNameSearch.create({
      dataSource: this,
    });

    this.nodeIdSearch = NodeIdSearch.create({
      dataSource: this,
    });
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
    try {
      set(this, 'searchString', string);

      const jobs = yield this.dataCaches.fetch('job');
      const nodes = yield this.dataCaches.fetch('node');

      set(this, 'jobs', jobs.toArray());
      set(this, 'nodes', nodes.toArray());

      const jobResults = this.jobSearch.listSearched.slice(0, MAXIMUM_RESULTS);

      const mergedNodeListSearched = this.nodeIdSearch.listSearched.concat(this.nodeNameSearch.listSearched).uniq();
      const nodeResults = mergedNodeListSearched.slice(0, MAXIMUM_RESULTS);

      return [
        {
          groupName: resultsGroupLabel('Jobs', jobResults, this.jobSearch.listSearched),
          options: jobResults,
        },
        {
          groupName: resultsGroupLabel('Clients', nodeResults, mergedNodeListSearched),
          options: nodeResults,
        },
      ];
    } catch (e) {
      // eslint-disable-next-line
      console.log('exception searching', e);
    }
  })
  searchOld;

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
    const allGroupResults = results.Matches.groups || [];

    const jobResults = allJobResults.slice(0, MAXIMUM_RESULTS);
    const nodeResults = allNodeResults.slice(0, MAXIMUM_RESULTS);
    const groupResults = allGroupResults.slice(0, MAXIMUM_RESULTS);

    jobResults.forEach(job => job.type = 'job');
    nodeResults.forEach(node => node.type = 'node');
    groupResults.forEach(group => group.type = 'group');

    return [
      {
        groupName: resultsGroupLabel('Jobs', jobResults, allJobResults),
        options: jobResults,
      },
      {
        groupName: resultsGroupLabel('Clients', nodeResults, allNodeResults),
        options: nodeResults,
      },
      {
        groupName: resultsGroupLabel('Task Groups', groupResults, allGroupResults),
        options: groupResults,
      },
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

@classic
class JobSearch extends EmberObject.extend(Searchable) {
  @computed
  get searchProps() {
    return ['id', 'name'];
  }

  @computed
  get fuzzySearchProps() {
    return ['name'];
  }

  @alias('dataSource.jobs') listToSearch;
  @alias('dataSource.searchString') searchTerm;

  fuzzySearchEnabled = true;
  includeFuzzySearchMatches = true;
}
@classic
class NodeNameSearch extends EmberObject.extend(Searchable) {
  @computed
  get searchProps() {
    return ['name'];
  }

  @computed
  get fuzzySearchProps() {
    return ['name'];
  }

  @alias('dataSource.nodes') listToSearch;
  @alias('dataSource.searchString') searchTerm;

  fuzzySearchEnabled = true;
  includeFuzzySearchMatches = true;
}

@classic
class NodeIdSearch extends EmberObject.extend(Searchable) {
  @computed
  get regexSearchProps() {
    return ['id'];
  }

  @alias('dataSource.nodes') listToSearch;
  @computed('dataSource.searchString')
  get searchTerm() {
    return `^${this.get('dataSource.searchString')}`;
  }

  exactMatchEnabled = false;
  regexEnabled = true;
}

function resultsGroupLabel(type, renderedResults, allResults) {
  let countString;

  if (renderedResults.length < allResults.length) {
    countString = `showing ${renderedResults.length} of ${allResults.length}`;
  } else {
    countString = renderedResults.length;
  }

  return `${type} (${countString})`;
}
