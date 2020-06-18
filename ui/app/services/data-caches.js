import Service, { inject as service } from '@ember/service';

export const COLLECTION_CACHE_DURATION = 60000; // one minute

export default class DataCachesService extends Service {
  @service router;
  @service store;
  @service system;

  collectionLastFetched = {};

  async fetch(modelName) {
    const modelNameToRoute = {
      job: 'jobs',
      node: 'clients',
    };

    const route = modelNameToRoute[modelName];
    const lastFetched = this.collectionLastFetched[modelName];
    const now = Date.now();

    if (this.router.isActive(route)) {
      // TODO Incorrect because it’s constantly being fetched by watchers, shouldn’t be marked as last fetched only on search
      this.collectionLastFetched[modelName] = now;
      return this.store.peekAll(modelName);
    } else if (lastFetched && now - lastFetched < COLLECTION_CACHE_DURATION) {
      return this.store.peekAll(modelName);
    } else {
      this.collectionLastFetched[modelName] = now;
      return this.store.findAll(modelName);
    }
  }
}
