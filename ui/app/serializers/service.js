import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class ServiceSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.ID = hash.ServiceName;
    return super.normalize(typeHash, hash);
  }
  normalizeResponse(store, typeClass, hash, ...args) {
    return super.normalizeResponse(
      store,
      typeClass,
      hash.mapBy('Services').flat() || [],
      ...args
    );
  }
}
