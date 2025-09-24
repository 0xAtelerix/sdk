# Pelagos Source-Available License (PSAL) v1.0

**Copyright © 2025, Pelagos Network**  
**All rights reserved.**

## 1. Definitions
- **“Software”** means the source code, object code, build scripts, configuration, interfaces, schemas, documentation, and other files in this repository and any derivative works you create from them.
- **“SDK”** means the developer kits, libraries, headers, and sample code distributed in or with the Software.
- **“Pelagos Network”** means the blockchain network operated under the “Pelagos” name that executes transactions exclusively via **Pelagos Core** and the **Pelagos Consensus** specified by Licensor.
- **“Permitted Use”** means: (a) local, non-production development, testing, benchmarking, or evaluation; and (b) production use **only on Pelagos Network**, provided all transactions are routed through Pelagos Core and finalized by Pelagos Consensus.
- **“Production Use”** means use to accept, relay, validate, settle or otherwise process transactions from or for third parties, or to run any public or private network or service for third parties.
- **“Competitive Offering”** means any product, network, protocol, service, or SDK that provides material functionality substantially similar to the Software, including forks or re-implementations, **other than** on Pelagos Network under this license.
- **“Copyleft Code”** means software governed by a copyleft license (e.g., GPL-2.0/3.0, AGPL-3.0, LGPL, MPL-2.0) that, when combined with other code, imposes reciprocal licensing or source-availability obligations.
- **“Contribution”** means any code, documentation, issue, comment, pull request, patch, or other material you submit to Licensor related to the Software (including code snippets embedded in comments).
- **“You”** means the individual or legal entity exercising rights under this license.

## 2. License Grant (Source-Available)
Subject to the terms below, Licensor grants You a non-exclusive, worldwide, non-transferable, non-sublicensable, royalty-free license to:
1. **Use** and **modify** the Software for the **Permitted Use**;
2. **Reproduce** and **distribute** unmodified or modified copies of the Software solely for the **Permitted Use** and only under this license.

No rights are granted for any other purpose.

## 3. Use Limitations (Field-of-Use)
You **must not**:
1. Use the Software for **Production Use** except on Pelagos Network as defined above.
2. Bypass, replace, or modify Pelagos Core or Pelagos Consensus when the Software is used in production.
3. Use the Software to operate, enable, or assist a **Competitive Offering**, including deploying or running an AMM/DEX, node/client, indexer, sequencer, or execution/consensus layer that is substantially similar in functionality to the Software **outside** Pelagos Network.
4. Offer the Software (or substantial functionality of it) as a hosted or managed service to third parties except when the service runs exclusively on Pelagos Network in compliance with Section 2.

## 4. Anti-Extraction / No Code-Fragment Reuse Outside Scope
Outside the **Permitted Use**, You must not copy, extract, adapt, or reuse **any portion** of the Software (including small code fragments, data structures, or interface definitions) in other software, documentation, or designs, except to the limited extent allowed by applicable law (e.g., fair use or reverse-engineering rights that cannot be waived).

## 5. Copyleft Interactions and Combined Works
a. If You create any module, package, or file in the repository that **imports, links to, incorporates, or is otherwise combined with Copyleft Code**, You must license **that module/package/file** under the applicable Copyleft License and comply with its requirements for source provision, notices, and reciprocal terms.  
b. Such Copyleft-governed modules **do not** change the license of other parts of the Software, which remain under this PSAL. You must keep Copyleft components clearly separated (directory or file-level separation and clear notices).  
c. You must not combine or distribute the Software in a way that would purport to impose Copyleft obligations on files that are not Copyleft Code, unless Licensor has agreed in writing.

## 6. Redistribution Conditions (Permitted Use Only)
Any redistribution (source or binary) must:
1. Be limited to the **Permitted Use**;
2. Include a copy of this license and all notices;
3. Preserve copyright, license, and attribution notices;
4. State clearly any changes You made;
5. Not use Licensor’s trademarks (including “Pelagos”) except for truthful, nominative reference.

## 7. Patents; Defensive Termination
Licensor grants You a non-exclusive, worldwide, royalty-free patent license under Licensor’s essential patent claims that are necessarily infringed by Your **Permitted Use** of the unmodified Software. If You or Your affiliate initiate a patent claim alleging that the Software or Pelagos Network infringes a patent, this license terminates immediately as to You.

## 8. Trademarks
This license does not grant rights to use Licensor’s names, logos, or trademarks (including “Pelagos”) except for factual, nominative use. Any goodwill arising from use of the marks belongs to Licensor.

## 9. Contributions: Assignment and License-Back
As a condition of submitting a **Contribution**:
1. **Assignment.** You hereby assign to Licensor all right, title, and interest (including copyright and, to the extent permitted, moral rights) in and to the Contribution upon submission.
2. **Fallback License.** If the above assignment is ineffective in any jurisdiction, You grant Licensor an exclusive, perpetual, irrevocable, worldwide, transferable, sublicensable, royalty-free license to use, reproduce, modify, distribute, make, have made, publicly perform and display, and relicense the Contribution for any purpose.
3. **License-Back to You.** Licensor grants You a non-exclusive, perpetual, worldwide, royalty-free license to use Your Contribution as part of the Software under this PSAL.
4. **DCO-Style Representations.** You represent that (a) You are the sole author and owner (or have permission to assign/license as above), (b) Your Contribution is provided under this PSAL, and (c) it does not knowingly infringe third-party rights.
5. **Feedback.** Non-code feedback that is not protectable by copyright is licensed to Licensor on a perpetual, irrevocable, worldwide, royalty-free basis.

## 10. Third-Party and FOSS Components
Third-party components included in the Software are licensed under their own licenses. This PSAL does not modify the terms of such components. You must comply with those licenses.

## 11. Term and Termination
This license is effective until terminated. It terminates automatically if You breach Sections 2–6 or 9, or upon the patent event in Section 7. If You cure an unintentional breach within 30 days after notice from Licensor, Licensor may, at its discretion, reinstate Your license. Upon termination, You must cease all use and distribution and destroy or return copies, except that Copyleft components You received from others remain under their respective licenses.

## 12. Disclaimer of Warranty
THE SOFTWARE IS PROVIDED “AS IS” AND “AS AVAILABLE,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING WITHOUT LIMITATION WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, NON-INFRINGEMENT, OR THAT THE SOFTWARE WILL BE ERROR-FREE OR SECURE.

## 13. Limitation of Liability
TO THE MAXIMUM EXTENT PERMITTED BY LAW, LICENSOR WILL NOT BE LIABLE FOR ANY INDIRECT, SPECIAL, INCIDENTAL, CONSEQUENTIAL, EXEMPLARY, OR PUNITIVE DAMAGES, OR FOR LOST PROFITS, REVENUE, DATA, OR GOODWILL, ARISING OUT OF OR RELATING TO THIS LICENSE OR THE SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY. LICENSOR’S TOTAL LIABILITY FOR ALL CLAIMS WILL NOT EXCEED USD $100.

## 14. Governing Law; Venue
This License shall be governed by, and construed in accordance with, the laws of the Cayman Islands.  
Any dispute arising out of or in connection with this License shall be subject to the exclusive jurisdiction of the courts of the Cayman Islands.

## 15. Entire Agreement; No Waiver; Severability
This license is the entire agreement concerning its subject matter. Failure to enforce any provision is not a waiver. If any provision is unenforceable, it will be modified to the minimum extent necessary to be enforceable, and the remainder will continue in effect.

---

> **NOTICE (informative; not part of the license):** This is a **source-available** license and is **not** an OSI-approved open-source license. Use outside the **Permitted Use** requires a separate commercial agreement with Licensor.
