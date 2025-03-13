# mip (ipconfig for macOS)

## Overview
mip is a lightweight utility designed to display essential network configuration information on macOS. It provides a simplified alternative to the verbose output of ifconfig by mimicking the concise style of Windows' ipconfig—showing only key details such as the IPv4 address, subnet mask, and default gateway.

## Key Features
- **Concise Output:** Eliminates unnecessary details to display only the essential network information.
- **Detailed Mode:** Use the `-all` flag to display additional details such as the physical (MAC) address, MTU, IPv6 addresses, and DNS server information.
- **Interface Selection:** The `-interface` flag allows you to view information for a specific network interface.
- **JSON Output:** The `-json` flag outputs results in JSON format, making integration with other tools and scripts straightforward.
- **Robust Error Handling:** Clear error messages are provided via stderr when issues arise in parsing network interface details, DNS server information, or default gateway data.

## Installation
Install mip using Homebrew with the following commands (brew deployment details are managed separately):

```bash
brew tap ashcastle/homebrew-mip
brew install ashcastle/mip/mipconfig
```

## Usage

After installation, simply run the mip command in your terminal to view your network configuration.

## Examples
	•	Basic Execution:

`mip`


	•	Detailed Information:

`mip -all`


	•	Display Specific Interface Information:

`mip -interface en0`


	•	JSON Format Output:

`mip -json`



## Contributing

This project is licensed under the GNU GPL and welcomes contributions from the community. Whether you’re reporting a bug, suggesting a feature, or improving documentation, your contributions are highly appreciated.

## License

This project is distributed under the GNU GPL.
