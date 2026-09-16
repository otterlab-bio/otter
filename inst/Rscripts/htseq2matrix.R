# htseq result to matrix
library(dplyr)
library(optparse)
library(readr)
option_list = list(make_option(c("--htseq_dir"), type = "character", default = NULL, help = "file, provided by default."),
                   make_option(c("--postfix"), type = "character", default = "_human.txt", help = "_human.txt, provided by default."),
                   make_option(c("--output_dir"), type = "character", default = NULL, help = "_human.txt, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
htseq_dir <- args$htseq_dir
postfix <- args$postfix
output_dir <- args$output_dir
htseqfiles <- list.files(path = htseq_dir,
                         pattern = postfix,
                         full.names = T)
origin_matrix <- NULL
for (i in 1:length(htseqfiles)){
  sampleid <- stringr::str_remove(htseqfiles[i],pattern = postfix)
  sampleid <- stringr::str_remove(sampleid,pattern = paste0(htseq_dir,"/"))
  message(glue::glue("processing {sampleid}"))
  temp_df <- readr::read_delim(htseqfiles[i],
                               delim = "\t", escape_double = FALSE,
                               col_names = FALSE, col_types = cols(X2 = col_number()),
                               trim_ws = TRUE)
  colnames(temp_df) <- c("Geneid",sampleid)
  if (is.null(origin_matrix)){
    origin_matrix <- temp_df
  }else{
    origin_matrix <- origin_matrix %>%
      dplyr::left_join(temp_df,by = "Geneid")
  }
}

message("transforming Entrezeid to Symbol")
if (postfix == "_human.txt"){
  load("data/Entrez_Gene_Id_db.rda")
  matrix_count <- origin_matrix %>%
    dplyr::right_join(data.frame(Geneid = Entrez_Gene_Id_db$ENSEMBL,
                                 Gene = Entrez_Gene_Id_db$SYMBOL),
                      by = "Geneid") %>%
    dplyr::select(-Geneid) %>%
    {
      temp <- .
      library(data.table)
      # 假设 matrix_count 是 data.frame 或 data.table
      setDT(temp)  # 原地转换为 data.table（无需复制）
      # 按 Gene 分组，对所有列（除 Gene）求 max 并忽略 NA
      temp <- temp[
        , lapply(.SD, max, na.rm = TRUE),
        by = Gene,
        .SDcols = -"Gene"  # .SDcols = 指定需处理的列（排除 Gene）
      ]
      as.data.frame(temp)
    } %>%
    dplyr::mutate(row_sum = rowSums(.[, -which(names(.) == "Gene")], na.rm = TRUE)) %>%
    dplyr::filter(row_sum != 0 & row_sum != -Inf & !is.na(row_sum)) %>%
    dplyr::select(-row_sum) %>%
    dplyr::relocate(Gene)
} else {
  load("data/Entrez_Gene_Id_db_mmu.rda")
  matrix_count <- origin_matrix %>%
    dplyr::right_join(data.frame(Geneid = Entrez_Gene_Id_db$UNIPROT,
                                 Gene = Entrez_Gene_Id_db$SYMBOL),
                      by = "Geneid") %>%
    dplyr::select(-Geneid) %>%
    {
      temp <- .
      library(data.table)
      # 假设 matrix_count 是 data.frame 或 data.table
      setDT(temp)  # 原地转换为 data.table（无需复制）
      # 按 Gene 分组，对所有列（除 Gene）求 max 并忽略 NA
      temp <- temp[
        , lapply(.SD, max, na.rm = TRUE),
        by = Gene,
        .SDcols = -"Gene"  # .SDcols = 指定需处理的列（排除 Gene）
      ]
      as.data.frame(temp)
    } %>%
    dplyr::mutate(row_sum = rowSums(.[, -which(names(.) == "Gene")], na.rm = TRUE)) %>%
    dplyr::filter(row_sum != 0 & row_sum != -Inf & !is.na(row_sum)) %>%
    dplyr::select(-row_sum) %>%
    dplyr::relocate(Gene)
}



write.table(matrix_count,
            file = paste0(output_dir,"/matrix_count.txt"),
            row.names = F,
            quote = F)
saveRDS(matrix_count,
        file = paste0(output_dir,"/matrix_count.RDS"))
matrix_norm <- matrix_count %>%
  tibble::column_to_rownames("Gene") %>%
  as.matrix() %>%
  {
    m <- .+1
    m <- log2(m)
    m
  } %>%
  as.data.frame() %>%
  dplyr::mutate_if(~!is.numeric(.), as.numeric) %>%
  tibble::rownames_to_column("Gene")

write.table(matrix_norm,
            file = paste0(output_dir,"/matrix_norm.txt"),
            row.names = F,
            quote = F)
saveRDS(matrix_norm,
        file = paste0(output_dir,"/matrix_norm.RDS"))
