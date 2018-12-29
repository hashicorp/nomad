/*
 * Apple System Management Controller (SMC) API from user space for Intel based
 * Macs. Works by talking to the AppleSMC.kext (kernel extension), the driver
 * for the SMC.
 *
 * smc.c
 * libsmc
 *
 * Copyright (C) 2014  beltex <https://github.com/beltex>
 *
 * Based off of fork from:
 * osx-cpu-temp <https://github.com/lavoiesl/osx-cpu-temp>
 *
 * With credits to:
 *
 * Copyright (C) 2006 devnull
 * Apple System Management Control (SMC) Tool
 *
 * Copyright (C) 2006 Hendrik Holtmann
 * smcFanControl <https://github.com/hholtmann/smcFanControl>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program; if not, write to the Free Software Foundation, Inc.,
 * 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */

#include <stdio.h>
#include <string.h>
#include "smc.h"


//------------------------------------------------------------------------------
// MARK: MACROS
//------------------------------------------------------------------------------


/**
Name of the SMC IOService as seen in the IORegistry. You can view it either via
command line with ioreg or through the IORegistryExplorer app (found on Apple's
developer site - Hardware IO Tools for Xcode)
*/
#define IOSERVICE_SMC "AppleSMC"


/**
IOService for getting machine model name
*/
#define IOSERVICE_MODEL "IOPlatformExpertDevice"


/**
SMC data types - 4 byte multi-character constants

Sources: See TMP SMC keys in smc.h

http://stackoverflow.com/questions/22160746/fpe2-and-sp78-data-types
*/
#define DATA_TYPE_UINT8  "ui8 "
#define DATA_TYPE_UINT16 "ui16"
#define DATA_TYPE_UINT32 "ui32"
#define DATA_TYPE_FLAG   "flag"
#define DATA_TYPE_FPE2   "fpe2"
#define DATA_TYPE_SFDS   "{fds"
#define DATA_TYPE_SP78   "sp78"


//------------------------------------------------------------------------------
// MARK: GLOBAL VARS
//------------------------------------------------------------------------------


/**
Our connection to the SMC
*/
static io_connect_t conn;


/**
Number of characters in an SMC key
*/
static const int SMC_KEY_SIZE = 4;


/**
Number of characters in a data type "key" returned from the SMC. See data type
macros.
*/
static const int DATA_TYPE_SIZE = 4;


//------------------------------------------------------------------------------
// MARK: ENUMS
//------------------------------------------------------------------------------


/**
Defined by AppleSMC.kext. See SMCParamStruct.

These are SMC specific return codes
*/
typedef enum {
    kSMCSuccess     = 0,
    kSMCError       = 1,
    kSMCKeyNotFound = 0x84
} kSMC_t;


/**
Defined by AppleSMC.kext. See SMCParamStruct.

Function selectors. Used to tell the SMC which function inside it to call.
*/
typedef enum {
    kSMCUserClientOpen  = 0,
    kSMCUserClientClose = 1,
    kSMCHandleYPCEvent  = 2,
    kSMCReadKey         = 5,
    kSMCWriteKey        = 6,
    kSMCGetKeyCount     = 7,
    kSMCGetKeyFromIndex = 8,
    kSMCGetKeyInfo      = 9
} selector_t;


//------------------------------------------------------------------------------
// MARK: STRUCTS
//------------------------------------------------------------------------------


/**
Defined by AppleSMC.kext. See SMCParamStruct.
*/
typedef struct {
    unsigned char  major;
    unsigned char  minor;
    unsigned char  build;
    unsigned char  reserved;
    unsigned short release;
} SMCVersion;


/**
Defined by AppleSMC.kext. See SMCParamStruct.
*/
typedef struct {
    uint16_t version;
    uint16_t length;
    uint32_t cpuPLimit;
    uint32_t gpuPLimit;
    uint32_t memPLimit;
} SMCPLimitData;


/**
Defined by AppleSMC.kext. See SMCParamStruct.

- dataSize : How many values written to SMCParamStruct.bytes
- dataType : Type of data written to SMCParamStruct.bytes. This lets us know how
             to interpret it (translate it to human readable)
*/
typedef struct {
    IOByteCount dataSize;
    uint32_t    dataType;
    uint8_t     dataAttributes;
} SMCKeyInfoData;


/**
Defined by AppleSMC.kext.

This is the predefined struct that must be passed to communicate with the
AppleSMC driver. While the driver is closed source, the definition of this
struct happened to appear in the Apple PowerManagement project at around
version 211, and soon after disappeared. It can be seen in the PrivateLib.c
file under pmconfigd.

https://www.opensource.apple.com/source/PowerManagement/PowerManagement-211/
*/
typedef struct {
    uint32_t       key;
    SMCVersion     vers;
    SMCPLimitData  pLimitData;
    SMCKeyInfoData keyInfo;
    uint8_t        result;
    uint8_t        status;
    uint8_t        data8;
    uint32_t       data32;
    uint8_t        bytes[32];
} SMCParamStruct;


/**
Used for returning data from the SMC.
*/
typedef struct {
    uint8_t  data[32];
    uint32_t dataType;
    uint32_t dataSize;
    kSMC_t   kSMC;
} smc_return_t;


//------------------------------------------------------------------------------
// MARK: HELPERS - TYPE CONVERSION
//------------------------------------------------------------------------------


/**
Convert data from SMC of fpe2 type to human readable.

:param: data Data from the SMC to be converted. Assumed data size of 2.
:returns: Converted data
*/
static unsigned int from_fpe2(uint8_t data[32])
{
    unsigned int ans = 0;

    // Data type for fan calls - fpe2
    // This is assumend to mean floating point, with 2 exponent bits
    // http://stackoverflow.com/questions/22160746/fpe2-and-sp78-data-types
    ans += data[0] << 6;
    ans += data[1] << 2;

    return ans;
}


/**
Convert to fpe2 data type to be passed to SMC.

:param: val Value to convert
:param: data Pointer to data array to place result
*/
static void to_fpe2(unsigned int val, uint8_t *data)
{
    data[0] = val >> 6;
    data[1] = (val << 2) ^ (data[0] << 8);
}


/**
Convert SMC key to uint32_t. This must be done to pass it to the SMC.

:param: key The SMC key to convert
:returns: uint32_t translation.
          Returns zero if key is not 4 characters in length.
*/
static uint32_t to_uint32_t(char *key)
{
    uint32_t ans   = 0;
    uint32_t shift = 24;

    // SMC key is expected to be 4 bytes - thus 4 chars
    if (strlen(key) != SMC_KEY_SIZE) {
        return 0;
    }

    for (int i = 0; i < SMC_KEY_SIZE; i++) {
        ans += key[i] << shift;
        shift -= 8;
    }

    return ans;
}


/**
For converting the dataType return from the SMC to human readable 4 byte
multi-character constant.
*/
static void to_string(uint32_t val, char *dataType)
{
    int shift = 24;

    for (int i = 0; i < DATA_TYPE_SIZE; i++) {
        // To get each char, we shift it into the lower 8 bits, and then & by
        // 255 to insolate it
        dataType[i] = (val >> shift) & 0xff;
        shift -= 8;
    }
}


//------------------------------------------------------------------------------
// MARK: HELPERS - TMP CONVERSION
//------------------------------------------------------------------------------


/**
Celsius to Fahrenheit
*/
static double to_fahrenheit(double tmp)
{
    // http://en.wikipedia.org/wiki/Fahrenheit#Definition_and_conversions
    return (tmp * 1.8) + 32;
}


/**
Celsius to Kelvin
*/
static double to_kelvin(double tmp)
{
    // http://en.wikipedia.org/wiki/Kelvin
    return tmp + 273.15;
}


//------------------------------------------------------------------------------
// MARK: "PRIVATE" FUNCTIONS
//------------------------------------------------------------------------------


/**
Make a call to the SMC

:param: inputStruct Struct that holds data telling the SMC what you want
:param: outputStruct Struct holding the SMC's response
:returns: I/O Kit return code
*/
static kern_return_t call_smc(SMCParamStruct *inputStruct,
                              SMCParamStruct *outputStruct)
{
    kern_return_t result;
    size_t inputStructCnt  = sizeof(SMCParamStruct);
    size_t outputStructCnt = sizeof(SMCParamStruct);

    result = IOConnectCallStructMethod(conn, kSMCHandleYPCEvent,
                                             inputStruct,
                                             inputStructCnt,
                                             outputStruct,
                                             &outputStructCnt);

    if (result != kIOReturnSuccess) {
        // IOReturn error code lookup. See "Accessing Hardware From Applications
        // -> Handling Errors" Apple doc
        result = err_get_code(result);
    }

    return result;
}


/**
Read data from the SMC

:param: key The SMC key
*/
static kern_return_t read_smc(char *key, smc_return_t *result_smc)
{
    kern_return_t result;
    SMCParamStruct inputStruct;
    SMCParamStruct outputStruct;

    memset(&inputStruct,  0, sizeof(SMCParamStruct));
    memset(&outputStruct, 0, sizeof(SMCParamStruct));
    memset(result_smc,    0, sizeof(smc_return_t));

    // First call to AppleSMC - get key info
    inputStruct.key = to_uint32_t(key);
    inputStruct.data8 = kSMCGetKeyInfo;

    result = call_smc(&inputStruct, &outputStruct);
    result_smc->kSMC = outputStruct.result;

    if (result != kIOReturnSuccess || outputStruct.result != kSMCSuccess) {
        return result;
    }

    // Store data for return
    result_smc->dataSize = outputStruct.keyInfo.dataSize;
    result_smc->dataType = outputStruct.keyInfo.dataType;    
    

    // Second call to AppleSMC - now we can get the data
    inputStruct.keyInfo.dataSize = outputStruct.keyInfo.dataSize;
    inputStruct.data8 = kSMCReadKey;

    result = call_smc(&inputStruct, &outputStruct);
    result_smc->kSMC = outputStruct.result;

    if (result != kIOReturnSuccess || outputStruct.result != kSMCSuccess) {
        return result;
    }

    memcpy(result_smc->data, outputStruct.bytes, sizeof(outputStruct.bytes));

    return result;
}


/**
Write data to the SMC.

:returns: IOReturn IOKit return code
*/
static kern_return_t write_smc(char *key, smc_return_t *result_smc)
{
    kern_return_t result;
    SMCParamStruct inputStruct;
    SMCParamStruct outputStruct;

    memset(&inputStruct,  0, sizeof(SMCParamStruct));
    memset(&outputStruct, 0, sizeof(SMCParamStruct));

    // First call to AppleSMC - get key info
    inputStruct.key = to_uint32_t(key);
    inputStruct.data8 = kSMCGetKeyInfo;

    result = call_smc(&inputStruct, &outputStruct);
    result_smc->kSMC = outputStruct.result;

    if (result != kIOReturnSuccess || outputStruct.result != kSMCSuccess) {
        return result;
    }

    // Check data is correct
    if (result_smc->dataSize != outputStruct.keyInfo.dataSize ||
        result_smc->dataType != outputStruct.keyInfo.dataType) {
        return kIOReturnBadArgument;
    }

    // Second call to AppleSMC - now we can write the data
    inputStruct.data8 = kSMCWriteKey;
    inputStruct.keyInfo.dataSize = outputStruct.keyInfo.dataSize;

    // Set data to write
    memcpy(inputStruct.bytes, result_smc->data, sizeof(result_smc->data));

    result = call_smc(&inputStruct, &outputStruct);
    result_smc->kSMC = outputStruct.result;

    return result;
}


/**
Get the model name of the machine.
*/
static kern_return_t get_machine_model(io_name_t model)
{
    io_service_t  service;
    kern_return_t result;
    
    service = IOServiceGetMatchingService(kIOMasterPortDefault,
                                          IOServiceMatching(IOSERVICE_MODEL));
    
    if (service == 0) {
        printf("ERROR: %s NOT FOUND\n", IOSERVICE_MODEL);
        return kIOReturnError;
    }

    // Get the model name
    result = IORegistryEntryGetName(service, model);
    IOObjectRelease(service);

    return result;
} 


//------------------------------------------------------------------------------
// MARK: "PUBLIC" FUNCTIONS
//------------------------------------------------------------------------------


kern_return_t open_smc(void)
{
    kern_return_t result;
    io_service_t service;

    service = IOServiceGetMatchingService(kIOMasterPortDefault,
                                          IOServiceMatching(IOSERVICE_SMC));

    if (service == 0) {
        // NOTE: IOServiceMatching documents 0 on failure
        printf("ERROR: %s NOT FOUND\n", IOSERVICE_SMC);
        return kIOReturnError;
    }

    result = IOServiceOpen(service, mach_task_self(), 0, &conn);
    IOObjectRelease(service);

    return result;
}


kern_return_t close_smc(void)
{
    return IOServiceClose(conn);
}


bool is_key_valid(char *key)
{
    bool ans = false;
    kern_return_t result;
    smc_return_t  result_smc;

    if (strlen(key) != SMC_KEY_SIZE) {
        printf("ERROR: Invalid key size - must be 4 chars\n");
        return ans;
    }

    // Try a read and see if it succeeds
    result = read_smc(key, &result_smc);

    if (result == kIOReturnSuccess && result_smc.kSMC == kSMCSuccess) {
        ans = true;
    }

    return ans;
}


double get_tmp(char *key, tmp_unit_t unit)
{
    kern_return_t result;
    smc_return_t  result_smc;

    result = read_smc(key, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 2   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_SP78))) {
        // Error
        return 0.0;
    }

    // TODO: Create from_sp78() convert function
    double tmp = result_smc.data[0];

    switch (unit) {
        case CELSIUS:
            break;
        case FAHRENHEIT:
            tmp = to_fahrenheit(tmp);
            break;
        case KELVIN:
            tmp = to_kelvin(tmp);
            break;
    }

    return tmp;
}


bool is_battery_powered(void)
{
    kern_return_t result;
    smc_return_t  result_smc;

    result = read_smc(BATT_PWR, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 1   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_FLAG))) {
        // Error
        return false;
    }

    return result_smc.data[0];
}


bool is_optical_disk_drive_full(void)
{
    kern_return_t result;
    smc_return_t  result_smc;

    result = read_smc(ODD_FULL, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 1   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_FLAG))) {
        // Error
        return false;
    }

    return result_smc.data[0];
}


//------------------------------------------------------------------------------
// MARK: FAN FUNCTIONS
//------------------------------------------------------------------------------


bool get_fan_name(unsigned int fan_num, fan_name_t name)
{
    char key[5];
    kern_return_t result;
    smc_return_t  result_smc;
    
    sprintf(key, "F%dID", fan_num);
    result = read_smc(key, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 16   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_SFDS))) {
      return false;
    }

  
    /*
    We know the data size is 16 bytes and the type is "{fds", a custom
    struct defined by the AppleSMC.kext. See TMP enum sources for the
    struct.
        
    The last 12 bytes contain the name of the fan, an array of chars, hence
    the loop range.
    */
    int index = 0; 
    for (int i = 4; i < 16; i++) {
        // Check if at the end (name may not be full 12 bytes)
        // Could check for 0 (null), but instead we check for 32 (space). This
        // is a hack to remove whitespace. :)
        if (result_smc.data[i] == 32) {
            break;
        }

        name[index] = result_smc.data[i];
        index++;
    }

    return true;
}


int get_num_fans(void)
{
    kern_return_t result;
    smc_return_t  result_smc;

    result = read_smc(NUM_FANS, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 1   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_UINT8))) {
        // Error
        return -1;
    }

    return result_smc.data[0];
}


unsigned int get_fan_rpm(unsigned int fan_num)
{
    char key[5];
    kern_return_t result;
    smc_return_t  result_smc;

    sprintf(key, "F%dAc", fan_num);
    result = read_smc(key, &result_smc);

    if (!(result == kIOReturnSuccess &&
          result_smc.dataSize == 2   &&
          result_smc.dataType == to_uint32_t(DATA_TYPE_FPE2))) {
        // Error
        return 0;
    }

    return from_fpe2(result_smc.data);
}


bool set_fan_min_rpm(unsigned int fan_num, unsigned int rpm, bool auth)
{
    // TODO: Add rpm val safety check
    char key[5];
    bool ans = false;
    kern_return_t result;
    smc_return_t  result_smc;

    memset(&result_smc, 0, sizeof(smc_return_t));

    // TODO: Don't use magic number
    result_smc.dataSize = 2;
    result_smc.dataType = to_uint32_t(DATA_TYPE_FPE2); 
    to_fpe2(rpm, result_smc.data);

    sprintf(key, "F%dMn", fan_num);
    result = write_smc(key, &result_smc);

    if (result == kIOReturnSuccess && result_smc.kSMC == kSMCSuccess) {
        ans = true;
    }

    return ans;
}
