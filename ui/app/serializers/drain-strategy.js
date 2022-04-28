import ApplicationSerializer from './application';

export default class DrainStrategy extends ApplicationSerializer {
  normalize(typeHash, hash) {
    // TODO API: finishedAt is always marshaled as a date even when unset.
    // To simplify things, unset it here when it's the empty date value.
    if (hash.ForceDeadline === '0001-01-01T00:00:00Z') {
      hash.ForceDeadline = null;
    }

    return super.normalize(typeHash, hash);
  }
}
