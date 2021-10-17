import ApplicationSerializer from './application';

export default class ScaleEventSerializer extends ApplicationSerializer {
  separateNanos = ['Time'];
  objectNullOverrides = ['Meta'];
}
