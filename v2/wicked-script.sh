#!/bin/sh
# Check for terminal.
 if ! [[ -t 0 ]] ; then
    # This script is not running in a terminal.
    echo "This script must be run from a terminal, by a user, and cannot be used in automation."
    exit 1;
 fi

# Greet the user.
clear
echo
echo "Welcome!"
echo
echo "This script will configure NPM to use the WICKED Artifactory repository for @wicked packages."
echo
echo "-------------------------------------------------------------------------------------------------"
echo "For security reasons, you must authenticate with an API key for TaaS NA Artifactory."
echo
echo "\tYou can generate or retrieve one from your Artifactory profile page:"
echo "\thttps://na.artifactory.swg-devops.com/artifactory/webapp/#/profile"
echo
echo "Do NOT use your IBM password at the prompt."
echo "-------------------------------------------------------------------------------------------------"
echo

# Get their email address.
printf "IBM Email Address: "
read EMAIL

# Validate the email address.
if ! [[ "${EMAIL}" =~ [a-zA-Z0-9._-]+@[a-z0-9.]*ibm\.com$ ]]; then
    echo "That does not appear to be a proper IBM email. Please try again."
    echo "If this message appears in error, please open an issue: https://github.ibm.com/wicked/help/issues/new"
    exit 2
fi

# Convert their email to lowercase for username.
USERNAME=`echo "${EMAIL}" | tr '[:upper:]' '[:lower:]'`

# Store current TTY settings.
OLD_TTY=`stty -g`

# Disable echo.
stty -echo

# Restore TTY settings if we exit early.
trap 'stty ${OLD_TTY}' EXIT

# Securely get their password.
printf "Artifactory API Key: "
read API_KEY

# Restore TTY settings and disable trap.
stty ${OLD_TTY}
trap - EXIT

# Start a new line.
echo

# Convert their password to base64
BASE64_PASSWORD=`printf ${API_KEY} | base64`

# Attempt to configure their NPM.
npm config set @wicked:registry https://na.artifactory.swg-devops.com/artifactory/api/npm/wicked-npm-local/
npm config set //na.artifactory.swg-devops.com/artifactory/api/npm/wicked-npm-local/:_password="${BASE64_PASSWORD}"
npm config set //na.artifactory.swg-devops.com/artifactory/api/npm/wicked-npm-local/:username="${USERNAME}"
npm config set //na.artifactory.swg-devops.com/artifactory/api/npm/wicked-npm-local/:email="${EMAIL}"
npm config set //na.artifactory.swg-devops.com/artifactory/api/npm/wicked-npm-local/:always-auth=true

# Verify their configuration
printf "\nTesting your configuration..."
npm cache clean > /dev/null 2>&1
if npm view @wicked/cli > /dev/null 2>&1; then
    echo "\rSUCCESS! NPM is now configured to use the WICKED Artifactory repository for @wicked packages.\n"
    exit 0
else
    echo "\rFAILED! Authentication was not successful. Please double check your credentials and try again.\n"
    exit 3
fi
