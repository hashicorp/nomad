import ApplicationSerializer from './application';

export default class TaskGroupScaleSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Events'];
}
