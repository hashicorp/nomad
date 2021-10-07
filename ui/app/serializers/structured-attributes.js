import ApplicationSerializer from './application';

export default class StructuredAttributes extends ApplicationSerializer {
  normalize(typeHash, hash) {
    return super.normalize(typeHash, { Raw: hash });
  }
}
