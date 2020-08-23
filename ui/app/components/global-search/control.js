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

  searchString = null;

  constructor() {
    super(...arguments);

    this.jobSearch = JobSearch.create({
      dataSource: this,
    });

    this.nodeSearch = NodeSearch.create({
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
      const nodeResults = this.nodeSearch.listSearched.slice(0, MAXIMUM_RESULTS);

      return [
        {
          groupName: resultsGroupLabel('Jobs', jobResults, this.jobSearch.listSearched),
          options: jobResults,
        },
        {
          groupName: resultsGroupLabel('Clients', nodeResults, this.nodeSearch.listSearched),
          options: nodeResults,
        },
      ];
    } catch (e) {
      // eslint-disable-next-line
      console.log('exception searching', e);
    }
  })
  search;

  @action
  open() {
    if (this.select) {
      this.select.actions.open();
    }
  }

  @action
  selectOption(model) {
    const itemModelName = model.constructor.modelName;

    if (itemModelName === 'job') {
      this.router.transitionTo('jobs.job', model.plainId, {
        queryParams: { namespace: model.get('namespace.name') },
      });
    } else if (itemModelName === 'node') {
      this.router.transitionTo('clients.client', model.id);
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
class NodeSearch extends EmberObject.extend(Searchable) {
  @computed
  get searchProps() {
    return ['id', 'name'];
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

function resultsGroupLabel(type, renderedResults, allResults) {
  let countString;

  if (renderedResults.length < allResults.length) {
    countString = `showing ${renderedResults.length} of ${allResults.length}`;
  } else {
    countString = renderedResults.length;
  }

  return `${type} (${countString})`;
}
