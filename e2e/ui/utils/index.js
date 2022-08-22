function Namespace(Name, ctx = {}) {
    return {
     Name,
     ...ctx
    }
   };
   
export const PROD_NAMESPACE = new Namespace(
     "prod",
     {
       Description: "Production API Servers",
       Meta: {
         contact: "platform-eng@example.com"
       },
       Quota: ""
     }
   );
 
export const DEV_NAMESPACE = new Namespace("dev");
 
export const OPERATOR_POLICY_JSON = JSON.stringify({
     "agent": [
       {
         "policy": "write"
       }
     ],
     "host_volume": [
       {
         "*": [
           {
             "policy": "write"
           }
         ]
       }
     ],
     "namespace": [
       {
         "*": [
           {
             "capabilities": [
               "alloc-node-exec",
               "csi-register-plugin"
             ],
             "policy": "write",
             "secure_variables": [
               {
                 "path": [
                   {
                     "*": [
                       {
                         "capabilities": [
                           "write",
                           "read",
                           "destroy",
                           "list"
                         ]
                       }
                     ]
                   }
                 ]
               }
             ]
           }
         ]
       }
     ],
     "node": [
       {
         "policy": "write"
       }
     ],
     "operator": [
       {
         "policy": "write"
       }
     ],
     "plugin": [
       {
         "policy": "read"
       }
     ],
     "quota": [
       {
         "policy": "write"
       }
     ]
});
 
export const DEV_POLICY_JSON = JSON.stringify({
     "agent": [
       {
         "policy": "read"
       }
     ],
     "host_volume": [
       {
         "*": [
           {
             "policy": "write"
           }
         ]
       }
     ],
     "namespace": [
       {
         "dev": [
           {
             "policy": "write",
             "secure_variables": [
               {
                 "path": [
                   {
                     "*": [
                       {
                         "capabilities": [
                           "list",
                           "read",
                           "write",
                           "destroy"
                         ]
                       }
                     ]
                   },
                   {
                     "system/*": [
                       {
                         "capabilities": [
                           "list",
                           "read"
                         ]
                       }
                     ]
                   }
                 ]
               }
             ]
           }
         ]
       },
       {
         "prod": [
           {
             "capabilities": [
               "list-jobs",
               "read-job",
               "read-logs"
             ],
             "secure_variables": [
               {
                 "path": [
                   {
                     "*": [
                       {
                         "capabilities": [
                           "list"
                         ]
                       }
                     ]
                   }
                 ]
               }
             ]
           }
         ]
       },
       {
         "*": [
           {
             "policy": "read",
             "secure_variables": [
               {
                 "path": [
                   {
                     "*": [
                       {
                         "capabilities": [
                           "list"
                         ]
                       }
                     ]
                   }
                 ]
               }
             ]
           }
         ]
       }
     ],
     "node": [
       {
         "policy": "read"
       }
     ],
     "operator": [
       {
         "policy": "read"
       }
     ]
});
 
export const ANON_POLICY_JSON = JSON.stringify({
     "agent": [
       {
         "policy": "read"
       }
     ],
     "node": [
       {
         "policy": "read"
       }
     ]
});
