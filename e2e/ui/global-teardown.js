import { client } from './api-client.js';

async function globalTeardown() {
  try {
      await client(`/namespace/prod`, {method: 'delete'});
      await client(`/namespace/dev`, {method: 'delete'});
      await client(`/acl/policy/operator`, {method: 'delete'});
      await client(`/acl/policy/dev`, {method: 'delete'});
      await client(`/acl/policy/anon`, {method: 'delete'});
      await client(`/acl/token/operator`, {method: 'delete'});
      await client(`/acl/token/dev`, {method: 'delete'});
      await client(`/acl/token/anon`, {method: 'delete'});
    } catch (e) {
      console.error('ERROR:  ', e)
    }  
};

export default globalTeardown;
