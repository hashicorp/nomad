import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { task } from 'ember-concurrency';
import EmberObject, { action, computed, set } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { run } from '@ember/runloop';
import Searchable from 'nomad-ui/mixins/searchable';
import classic from 'ember-classic-decorator';

const SLASH_KEY = 191;

@tagName('')
export default class GlobalSearchControl extends Component {
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

    // FIXME are more and/or other-approach exceptions needed?
    if (targetElementName != 'input' && targetElementName != 'textarea') {
      if (e.keyCode === SLASH_KEY) {
        e.preventDefault();
        this.open();
      }
    }
  }

  didInsertElement() {
    this.set('_keyDownHandler', this.keyDownHandler.bind(this));
    document.addEventListener('keydown', this._keyDownHandler);
  }

  willDestroyElement() {
    document.removeEventListener('keydown', this._keyDownHandler);
  }

  @task(function*(string) {
    try {
      set(this, 'searchString', string);

      // FIXME no need to fetch on every search!
      const jobs = yield this.fetchJobs();
      const nodes = yield this.fetchNodes();

      set(this, 'jobs', jobs.toArray());
      set(this, 'nodes', nodes.toArray());

      const jobResults = this.jobSearch.listSearched;
      const nodeResults = this.nodeSearch.listSearched;

      return [
        {
          groupName: `Jobs (${jobResults.length})`,
          options: jobResults,
        },
        {
          groupName: `Clients (${nodeResults.length})`,
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
      this.router.transitionTo('jobs.job', model.name);
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
      run.next(() => {
        select.actions.open();
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

  async fetchJobs() {
    if (this.router.isActive('jobs')) {
      return this.store.peekAll('job');
    } else {
      return this.store.findAll('job');
    }
  }

  async fetchNodes() {
    if (this.router.isActive('clients')) {
      return this.store.peekAll('node');
    } else {
      return this.store.findAll('node');
    }
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
}
