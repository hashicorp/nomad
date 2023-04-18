import ApplicationSerializer from './application';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroupScaleSerializer extends ApplicationSerializer {
  arrayNullOverrides = ['Events'];
}
