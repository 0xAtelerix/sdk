use solana_program::{
    account_info::AccountInfo,
    entrypoint,
    entrypoint::ProgramResult,
    instruction::{AccountMeta, Instruction},
    msg,
    program::invoke,
    program_error::ProgramError,
    pubkey::Pubkey,
};

entrypoint!(process_instruction);

/// Pelagos: Cross-Chain Router Program
///
/// Forwards ETX payload to dynamic appchain program.
///
/// Tx Keys: [0]=payer (signer/writable), [1]=appchain_program (readonly/executable), [2..]=appchain-specific accounts
/// Data: Opaque bytes (forwarded as-is to appchain).
pub fn process_instruction(
    _program_id: &Pubkey,
    accounts: &[AccountInfo],
    instruction_data: &[u8],
) -> ProgramResult {
    // Validate instruction data is not empty
    if instruction_data.is_empty() {
        msg!("❌ Pelagos: Empty instruction data");
        return Err(ProgramError::InvalidInstructionData);
    }

    // Validate we have at least payer and appchain program accounts
    if accounts.len() < 2 {
        msg!("❌ Pelagos: Insufficient accounts (need >=2: payer + appchain_program)");
        return Err(ProgramError::NotEnoughAccountKeys);
    }

    // Validate payer account is signer
    if !accounts[0].is_signer {
        msg!("❌ Pelagos: Payer must be signer");
        return Err(ProgramError::MissingRequiredSignature);
    }

    // accounts[0] = payer (signer, writable)
    // accounts[1] = appchain program
    // accounts[2..] = appchain-specific accounts
    let appchain_program_id = accounts[1].key;

    msg!("🔄 Pelagos: Forwarding to appchain {}", appchain_program_id);

    // Create CPI instruction with accounts for the appchain program
    let appchain_accounts: Vec<_> = accounts[2..]
        .iter()
        .map(|acc| AccountMeta {
            pubkey: *acc.key,
            is_signer: acc.is_signer,
            is_writable: acc.is_writable,
        }).collect();

    let cpi_instruction = Instruction {
        program_id: *appchain_program_id,
        accounts: appchain_accounts,
        data: instruction_data.to_vec(),
    };

    // Execute CPI call with all accounts (appchain program + appchain accounts)
    invoke(&cpi_instruction, &accounts[1..])?;

    msg!("✅ Pelagos: CPI completed");
    Ok(())
}
