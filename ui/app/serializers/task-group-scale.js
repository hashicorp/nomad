import ApplicationSerializer from './application';

export default class TaskGroupScaleSerializer extends ApplicationSerializer {
  normalize(typeHash, hash) {
    if (!hash.Events) {
      hash.Events = [];
    }

    return super.normalize(typeHash, hash);
  }
}
