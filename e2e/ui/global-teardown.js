import { client } from './api-client.js';

async function globalTeardown() {
  try {
      await client(`/namespace/prod`, {method: 'delete'});
      await client(`/namespace/dev`, {method: 'delete'});
      await client(`/acl/policy/operator`, {method: 'delete'});
      await client(`/acl/policy/dev`, {method: 'delete'});
      const {data: tokens} = await client(`/acl/tokens`);
      console.log('TOKENZS\n\n\n\n', tokens)
      tokens.forEach(token => {
        if (token.Type === 'client') {
          await client(`/acl/token/${token.AccessorID}`, {method: 'delete'});
        }
      })
    } catch (e) {
      console.error('ERROR:  ', e)
    }  
};

export default globalTeardown;
