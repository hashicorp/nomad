import { computed, set } from '@ember/object';
import { readOnly } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import { copy } from 'ember-copy';
import Service from '@ember/service';

let list = {};

export default class WatchListService extends Service {
  @service store;

  jobUpdateCount = 0;

  @computed
  get _list() {
    return copy(list, true);
  }

  @readOnly('_list') list;

  constructor() {
    super(...arguments);
    list = {};
  }

  getIndexFor(url) {
    return list[url] || 1;
  }

  setIndexFor(url, value) {
    list[url] = +value;
  }

  notifyController() {
    set(this, 'jobUpdateCount', this.jobUpdateCount + 1);
  }
}
