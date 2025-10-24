use solana_program::{
    account_info::AccountInfo,
    entrypoint,
    entrypoint::ProgramResult,
    msg,
    program::invoke_signed,
    program_error::ProgramError,
    pubkey::Pubkey,
};
use spl_token::instruction as token_instruction;

entrypoint!(process_instruction);

/// Appchain Token Minting Program
///
/// Payload format (40 bytes): [recipient:32][amount:8]
/// Accounts (5 total): [program, mint, recipient_ata, mint_authority_pda, token_program]
/// Note: accounts[0] is the program itself in this example
pub fn process_instruction(
    program_id: &Pubkey,
    accounts: &[AccountInfo],
    instruction_data: &[u8],
) -> ProgramResult {
    // Validate input
    if instruction_data.len() != 40 {
        msg!(
            "❌ Appchain: Invalid instruction data length: {}",
            instruction_data.len()
        );
        return Err(ProgramError::InvalidInstructionData);
    }
    if accounts.len() < 5 {
        msg!("❌ Appchain: Insufficient accounts: got {}, need 5", accounts.len());
        return Err(ProgramError::NotEnoughAccountKeys);
    }

    // Parse payload
    let recipient_pubkey = Pubkey::try_from(&instruction_data[0..32])
        .map_err(|_| ProgramError::InvalidInstructionData)?;
    let amount = u64::from_le_bytes(
        instruction_data[32..40]
            .try_into()
            .map_err(|_| ProgramError::InvalidInstructionData)?,
    );

    // Get accounts (skip [0] = program itself)
    let mint_account = &accounts[1];
    let recipient_token_account = &accounts[2];
    let mint_authority = &accounts[3];
    let token_program = &accounts[4];

    // Derive PDA
    let (expected_authority, authority_bump) =
        Pubkey::find_program_address(&[b"mint_authority"], program_id);

    if mint_authority.key != &expected_authority {
        msg!("❌ Appchain: Invalid mint authority PDA");
        return Err(ProgramError::InvalidSeeds);
    }

    // Mint tokens via CPI
    let mint_ix = token_instruction::mint_to(
        token_program.key,
        mint_account.key,
        recipient_token_account.key,
        mint_authority.key,
        &[],
        amount,
    )?;

    invoke_signed(
        &mint_ix,
        &[
            mint_account.clone(),
            recipient_token_account.clone(),
            mint_authority.clone(),
            token_program.clone(),
        ],
        &[&[b"mint_authority", &[authority_bump]]],
    )?;

    msg!("✅ Appchain: Minted {} tokens to {}", amount, recipient_pubkey);
    Ok(())
}
