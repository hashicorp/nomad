import ApplicationSerializer from './application';

export default class Policy extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.ID = hash.Name;
    return super.normalize(typeHash, hash);
  }
}
