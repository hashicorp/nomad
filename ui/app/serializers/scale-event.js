import ApplicationSerializer from './application';

export default class ScaleEventSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    hash.TimeNanos = hash.Time % 1000000;
    hash.Time = Math.floor(hash.Time / 1000000);

    return super.normalize(typeHash, hash);
  }
}
