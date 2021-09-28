# j

{
  "data": [ //metrics
    {
      "eventId": "unique guid per individual event", //metricId
      "start": 1485907200001, // internvalStart
      "end": 1485910800000, //intervalEnd
      "accountId": "RHM account id" //accountId
      "additionalAttributes": {}, //extra variables - domain, kind
      "measuredUsage": [ //MetricExtended
        {
          "metricId": "VIRTUAL_PROCESSOR_CORE",
          "value": 20,
          "additionalAttributes": {}
        },
        {
          "metricId": "PROCESSOR_VALUE_UNIT",
          "value": 6000,
          "additionalAttributes": {}
        }
      ]
    }
  ],
  "metadata": {}
}

{
  "data": [ //metrics
    {
      "eventId": "unique guid per individual event", //metricId
      "start": 1485907200001, // internvalStart
      "end": 1485910800000, //intervalEnd
      "accountId": "RHM account id" //accountId
      "additionalAttributes": {}, //extra variables - domain, kind
      "measuredUsage": [ //MetricExtended
        {
          "metricId": "VIRTUAL_PROCESSOR_CORE",
          "value": 20,
          "additionalAttributes": { "var" : "a", "value" : 20 }
        },
        {
          "metricId": "VIRTUAL_PROCESSOR_CORE",
          "value": 6000,
          "additionalAttributes": { "var" : "b", "value" : 6000 }
        }
      ]
    }
  ],
  "metadata": {}
}
