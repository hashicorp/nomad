import ApplicationSerializer from './application';

export default class ServiceSerializer extends ApplicationSerializer {
  attrs = {
    connect: 'Connect',
  };

  arrayNullOverrides = ['Tags'];
}
